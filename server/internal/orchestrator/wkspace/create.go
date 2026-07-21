package wkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// InitTaskWorkspace creates the directory structure and metadata for a new task workspace.
func (m *Manager) InitTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	ws := m.GetTaskWorkspace(task)

	dirs := []string{
		ws.Root,
		ws.SpecsDir,
		ws.ContextDir,
		ws.ArtifactsDir,
		filepath.Join(ws.ArtifactsDir, "checkpoints"),
		filepath.Join(ws.ArtifactsDir, "diffs"),
		filepath.Join(ws.ArtifactsDir, "tests"),
		ws.LogsDir,
		filepath.Join(ws.LogsDir, "llm"),
		ws.PRDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	var projectRepos []models.Repository
	if m.Repositories != nil {
		var err error
		projectRepos, err = m.Repositories.ListByProjectID(ctx, task.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("list project repositories: %w", err)
		}
	}

	repos := []models.RepoWorkspace{}
	for _, pr := range projectRepos {
		if task.RepositoryID != nil && *task.RepositoryID != pr.ID {
			continue
		}

		parts := strings.Split(pr.URL, "/")
		repoName := parts[len(parts)-1]
		repoName = strings.TrimSuffix(repoName, ".git")

		defaultBranch := pr.Branch
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		repoWS := models.RepoWorkspace{
			RepoID:        pr.ID,
			Name:          repoName,
			URL:           pr.URL,
			DefaultBranch: defaultBranch,
			Status: models.RepoWorkspaceStatus{
				MergeStatus: models.MergeStatusPending,
				TestStatus:  models.TestStatusPending,
			},
			Paths: models.RepoWorkspacePaths{
				Main:      paths.NewOSWorkspacePaths("").RepoMainRelative(repoName),
				Worktrees: make(map[string]string),
			},
			Branches: models.RepoWorkspaceBranches{
				Integration: paths.DeriveBranchName(task.ID, task.Title),
				Role:        make(map[string]string),
			},
		}

		repos = append(repos, repoWS)
	}
	ws.Repos = repos

	taskSnap := models.TaskStateSnapshot{
		TaskID:      task.ID,
		ProjectID:   task.ProjectID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		Complexity:  task.Complexity,
		SpecStatus:  task.SpecStatus,
		Labels:      task.Labels,
	}
	taskJSONPath := filepath.Join(ws.Root, "task.json")
	taskBytes, err := json.MarshalIndent(taskSnap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal task snapshot: %w", err)
	}
	if err := os.WriteFile(taskJSONPath, taskBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write task.json: %w", err)
	}

	meta := models.TaskWorkspaceMetadata{
		WorkspaceVersion: 1,
		Repos:            repos,
	}
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaJSONPath, metaBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write metadata.json: %w", err)
	}

	return ws, nil
}

// EnsureWorkspaceCloned guarantees repos are cloned and ready for execution.
func (m *Manager) EnsureWorkspaceCloned(ctx context.Context, task *models.Task, agent *models.Agent, jobID string) error {
	if m.Repositories == nil {
		return fmt.Errorf("repositories lookup not configured")
	}
	if m.GitOps == nil {
		return fmt.Errorf("gitops client not configured")
	}

	ws, err := m.LoadTaskWorkspace(ctx, task)
	if err != nil {
		ws, err = m.InitTaskWorkspace(ctx, task)
		if err != nil {
			return fmt.Errorf("failed to init task workspace: %w", err)
		}
	}

	if err := m.AcquireWorkspaceLock(ctx, task, jobID); err != nil {
		return fmt.Errorf("failed to acquire workspace lock: %w", err)
	}

	checkpoints, cpErr := m.Workflows.ListCheckpoints(ctx, task.ID)
	hasSuccessfulCodeStep := false
	completedSteps := make(map[string]bool)
	if cpErr == nil && len(checkpoints) > 0 {
		for _, cp := range checkpoints {
			var state map[string]any
			if json.Unmarshal(cp.State, &state) == nil {
				if status, _ := state["status"].(string); status == workflow.StepStatusSuccess {
					completedSteps[cp.Step] = true
					if cp.Step == workflow.StepCodeBackend || cp.Step == workflow.StepCodeFrontend || cp.Step == workflow.StepFix || cp.Step == workflow.StepMerge {
						hasSuccessfulCodeStep = true
					}
				}
			}
		}
	}

	var workspaceRestored bool

	for i, rWS := range ws.Repos {
		repoAbsPath := filepath.Join(ws.Root, rWS.Paths.Main)
		gitDir := filepath.Join(repoAbsPath, ".git")

		workspaceExists := false
		if stat, err := os.Stat(gitDir); err == nil && stat.IsDir() {
			workspaceExists = true
		}

		if workspaceExists {
			if !hasSuccessfulCodeStep {
				if err := ResetExistingWorkspace(ctx, repoAbsPath); err != nil {
					return fmt.Errorf("reset existing workspace at %s: %w", repoAbsPath, err)
				}
			}
		} else {
			os.RemoveAll(repoAbsPath)
			if err := os.MkdirAll(filepath.Dir(repoAbsPath), 0o755); err != nil {
				return fmt.Errorf("create repo parent dir: %w", err)
			}
			clonedBranch, err := m.GitOps.CloneForTask(ctx, rWS.URL, rWS.DefaultBranch, repoAbsPath)
			if err != nil {
				return fmt.Errorf("clone repo %s: %w", rWS.Name, err)
			}
			if clonedBranch != "" {
				ws.Repos[i].DefaultBranch = clonedBranch
			}
			workspaceRestored = true
		}

		m.populateGoModulesCache(ctx, task.ID, repoAbsPath)

		ws.Repos[i].Branches.Integration = paths.DeriveBranchName(task.ID, task.Title)
	}

	if err := m.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if hasSuccessfulCodeStep && workspaceRestored {
		if m.Artifacts != nil {
			if arts, errArts := m.Artifacts.ListByTaskID(ctx, task.ID); errArts == nil {
				for _, art := range arts {
					if !completedSteps[art.Step] {
						continue
					}
					if art.Type == "patch" {
						var patchText string
						if json.Unmarshal(art.Payload, &patchText) == nil && patchText != "" {
							m.Log(ctx, task.ID, nil, "info", fmt.Sprintf("Restoring checkpoint patch for step %s...", art.Step))
							if errApply := m.ApplyPatch(ctx, task, agent, art.Step+"_restore", patchText, ""); errApply != nil {
								m.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("Failed to restore patch for step %s: %v", art.Step, errApply))
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (m *Manager) populateGoModulesCache(ctx context.Context, taskID string, repoPath string) {
	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.Name() == "go.mod" {
			dir := filepath.Dir(path)
			m.Log(ctx, taskID, nil, "info", fmt.Sprintf("Pre-downloading dependencies for Go module at %s on host...", dir))
			cmd := exec.CommandContext(ctx, "go", "mod", "download")
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				m.Log(ctx, taskID, nil, "warn", fmt.Sprintf("go mod download failed on host for %s: %v", dir, err))
			}
		}
		return nil
	})
}
