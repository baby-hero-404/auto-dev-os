package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	orchestratorworkspace "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// GetTaskWorkspace returns the workspace layout for a task.
func (o *Orchestrator) GetTaskWorkspace(task *models.Task) *models.TaskWorkspace {
	return orchestratorworkspace.GetTaskWorkspace(o.workspaceRoot, task)
}

// InitTaskWorkspace creates the directory structure and metadata for a new task workspace.
func (o *Orchestrator) InitTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	ws := o.GetTaskWorkspace(task)

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
	if o.repositories != nil {
		var err error
		projectRepos, err = o.repositories.ListByProjectID(ctx, task.ProjectID)
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

		repoWS := models.RepoWorkspace{
			RepoID:        pr.ID,
			Name:          repoName,
			URL:           pr.URL,
			DefaultBranch: pr.Branch,
			Status: models.RepoWorkspaceStatus{
				MergeStatus: models.MergeStatusPending,
				TestStatus:  models.TestStatusPending,
			},
			Paths: models.RepoWorkspacePaths{
				Main:      filepath.Join("code", "repos", repoName, "main"),
				Worktrees: make(map[string]string),
			},
			Branches: models.RepoWorkspaceBranches{
				Integration: fmt.Sprintf("feature/%s", task.ID),
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

// LoadTaskWorkspace loads workspace metadata from disk, auto-initializing if needed.
func (o *Orchestrator) LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	ws := o.GetTaskWorkspace(task)
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := os.ReadFile(metaJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			if _, statErr := os.Stat(ws.Root); statErr == nil {
				return o.InitTaskWorkspace(ctx, task)
			}
			return nil, fmt.Errorf("metadata.json not found and workspace does not exist: %w", err)
		}
		return nil, err
	}

	var meta models.TaskWorkspaceMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata.json: %w", err)
	}

	ws.Repos = meta.Repos
	return ws, nil
}

// SaveTaskWorkspaceMetadata persists workspace repo metadata to metadata.json.
func (o *Orchestrator) SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error {
	meta := models.TaskWorkspaceMetadata{
		WorkspaceVersion: 1,
		Repos:            ws.Repos,
	}
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaJSONPath, metaBytes, 0o644)
}

// ResolveRepoWorkspace finds a specific repo workspace by ID.
func (o *Orchestrator) ResolveRepoWorkspace(ctx context.Context, task *models.Task, repoID string) (*models.RepoWorkspace, error) {
	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil {
		return nil, err
	}
	for i := range ws.Repos {
		if ws.Repos[i].RepoID == repoID {
			return &ws.Repos[i], nil
		}
	}
	return nil, fmt.Errorf("repo %s not found in workspace", repoID)
}

// FindRepoWorkspaceByPath matches a filesystem path to a repo workspace.
func (o *Orchestrator) FindRepoWorkspaceByPath(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error) {
	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for i := range ws.Repos {
		rWS := &ws.Repos[i]
		mainAbs := filepath.Join(ws.Root, rWS.Paths.Main)
		if mainAbs == absPath || strings.HasPrefix(absPath, mainAbs) {
			return rWS, nil
		}
		for _, wtRel := range rWS.Paths.Worktrees {
			wtAbs := filepath.Join(ws.Root, wtRel)
			if wtAbs == absPath || strings.HasPrefix(absPath, wtAbs) {
				return rWS, nil
			}
		}
		repoRootAbs := filepath.Join(ws.Root, "code", "repos", rWS.Name)
		if rel, errRel := filepath.Rel(repoRootAbs, absPath); errRel == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rWS, nil
		}
	}
	if len(ws.Repos) == 1 {
		return &ws.Repos[0], nil
	}
	return nil, fmt.Errorf("no repository matching path %s found in workspace", path)
}

// ensureWorkspaceCloned guarantees repos are cloned and ready for execution.
func (o *Orchestrator) ensureWorkspaceCloned(ctx context.Context, task *models.Task, agent *models.Agent, jobID string) error {
	if o.repositories == nil {
		return fmt.Errorf("repositories lookup not configured")
	}
	if o.gitOps == nil {
		return fmt.Errorf("gitops client not configured")
	}

	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil {
		ws, err = o.InitTaskWorkspace(ctx, task)
		if err != nil {
			return fmt.Errorf("failed to init task workspace: %w", err)
		}
	}

	if err := o.acquireWorkspaceLock(ctx, task, jobID); err != nil {
		return fmt.Errorf("failed to acquire workspace lock: %w", err)
	}

	checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID)
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
				if err := resetExistingWorkspace(ctx, repoAbsPath); err != nil {
					return fmt.Errorf("reset existing workspace at %s: %w", repoAbsPath, err)
				}
			}
		} else {
			os.RemoveAll(repoAbsPath)
			if err := os.MkdirAll(filepath.Dir(repoAbsPath), 0o755); err != nil {
				return fmt.Errorf("create repo parent dir: %w", err)
			}
			if _, err := o.gitOps.CloneForTask(ctx, rWS.URL, rWS.DefaultBranch, repoAbsPath); err != nil {
				return fmt.Errorf("clone repo %s: %w", rWS.Name, err)
			}
			workspaceRestored = true
		}

		ws.Repos[i].Branches.Integration = fmt.Sprintf("feature/%s", task.ID)
	}

	if err := o.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if hasSuccessfulCodeStep && workspaceRestored {
		if o.artifacts != nil {
			if arts, errArts := o.artifacts.ListByTaskID(ctx, task.ID); errArts == nil {
				for _, art := range arts {
					if !completedSteps[art.Step] {
						continue
					}
					if art.Type == "patch" {
						var patchText string
						if json.Unmarshal(art.Payload, &patchText) == nil && patchText != "" {
							o.log(ctx, task.ID, nil, "info", fmt.Sprintf("Restoring checkpoint patch for step %s...", art.Step))
							if errApply := o.applyPatch(ctx, task, agent, art.Step+"_restore", patchText, ""); errApply != nil {
								o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("Failed to restore patch for step %s: %v", art.Step, errApply))
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// cleanupWorkspaceAfterFinalState releases locks and optionally prunes workspace.
func (o *Orchestrator) cleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	o.releaseWorkspaceLock(taskID)

	if o.retention.Retention != 0 {
		return
	}
	if err := o.partialCleanupWorkspace(ctx, taskID); err != nil {
		observability.Warn(ctx, "workspace partial cleanup failed", "task_id", taskID, "error", err)
	} else {
		observability.Info(ctx, "workspace partially cleaned after final state", "task_id", taskID)
	}
}

// partialCleanupWorkspace removes worktrees while preserving diffs.
func (o *Orchestrator) partialCleanupWorkspace(ctx context.Context, taskID string) error {
	o.releaseWorkspaceLock(taskID)

	root := sandbox.WorkspacePath(o.workspaceRoot, taskID)
	codeDir := filepath.Join(root, "code", "repos")

	repos, err := os.ReadDir(codeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, rEntry := range repos {
		if !rEntry.IsDir() {
			continue
		}
		repoName := rEntry.Name()
		wtParentDir := filepath.Join(codeDir, repoName, "worktrees")
		worktrees, err := os.ReadDir(wtParentDir)
		if err != nil {
			continue
		}

		mainAbs := filepath.Join(codeDir, repoName, "main")

		for _, wtEntry := range worktrees {
			if !wtEntry.IsDir() {
				continue
			}
			role := wtEntry.Name()
			wtAbs := filepath.Join(wtParentDir, role)

			// Check git status to see if there are uncommitted changes
			statusCmd := exec.CommandContext(ctx, "git", "-C", wtAbs, "status", "--porcelain")
			statusOut, statusErr := statusCmd.CombinedOutput()
			if statusErr == nil && len(strings.TrimSpace(string(statusOut))) > 0 {
				// Capture both staged and unstaged modifications
				diffCmd := exec.CommandContext(ctx, "git", "-C", wtAbs, "diff", "HEAD")
				diffOut, diffErr := diffCmd.CombinedOutput()
				if diffErr != nil {
					return fmt.Errorf("failed to capture diff for worktree %s: %w", wtAbs, diffErr)
				}

				statusClean := strings.TrimSpace(string(statusOut))
				fullDiffContent := []byte(fmt.Sprintf("=== Worktree Status ===\n%s\n\n=== Diffs ===\n%s", statusClean, string(diffOut)))

				diffDir := filepath.Join(root, "artifacts", "diffs")
				if err := os.MkdirAll(diffDir, 0o755); err != nil {
					return fmt.Errorf("failed to create diff dir: %w", err)
				}
				diffPath := filepath.Join(diffDir, fmt.Sprintf("cleanup-%s-%s.diff", repoName, role))
				if err := os.WriteFile(diffPath, fullDiffContent, 0o644); err != nil {
					return fmt.Errorf("failed to write cleanup diff: %w", err)
				}
			}

			// Prune worktree using git worktree remove
			pruneCmd := exec.CommandContext(ctx, "git", "-C", mainAbs, "worktree", "remove", wtAbs, "--force")
			if err := pruneCmd.Run(); err != nil {
				// Fallback to manual removal if git worktree remove fails
				if errRemove := os.RemoveAll(wtAbs); errRemove != nil {
					return fmt.Errorf("failed to remove worktree path %s: %w", wtAbs, errRemove)
				}
			}
		}
	}

	// Update metadata.json if it exists and can be loaded
	if o.tasks != nil {
		if task, err := o.tasks.GetByID(ctx, taskID); err == nil {
			if ws, errLoad := o.LoadTaskWorkspace(ctx, task); errLoad == nil {
				for i := range ws.Repos {
					ws.Repos[i].Paths.Worktrees = make(map[string]string)
					ws.Repos[i].Branches.Role = make(map[string]string)
				}
				_ = o.SaveTaskWorkspaceMetadata(task, ws)
			}
		}
	}

	return nil
}

// RemoveWorkspace deletes the entire workspace directory for a task.
func (o *Orchestrator) RemoveWorkspace(taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task id is required")
	}
	o.releaseWorkspaceLock(taskID)
	root := o.workspaceRoot
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(sandbox.WorkspacePath(root, taskID))
	if err != nil {
		return err
	}
	if targetAbs == rootAbs {
		return fmt.Errorf("refusing to remove workspace root")
	}
	rootPrefix := rootAbs + string(os.PathSeparator)
	if !strings.HasPrefix(targetAbs, rootPrefix) {
		return fmt.Errorf("workspace path escapes root")
	}
	return os.RemoveAll(targetAbs)
}

func resetExistingWorkspace(ctx context.Context, localPath string) error {
	commands := [][]string{
		{"git", "-C", localPath, "reset", "--hard"},
		{"git", "-C", localPath, "clean", "-fdx"},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, string(output))
		}
	}
	return nil
}

func getRoleFromSuffix(suffix string) string {
	suffix = strings.TrimPrefix(suffix, "-")
	suffix = strings.TrimSuffix(suffix, "-worktree")
	switch suffix {
	case "be", "backend":
		return "backend"
	case "fe", "frontend":
		return "frontend"
	case "fix":
		return "fix"
	default:
		return suffix
	}
}
