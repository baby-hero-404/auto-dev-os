package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) StartWorkspacePruner(ctx context.Context) {
	if o.retention.Retention <= 0 {
		return
	}
	interval := o.retention.Interval
	if interval <= 0 {
		interval = time.Hour
	}

	if removed, err := o.pruneWorkspaces(ctx); err != nil {
		observability.Warn(ctx, "workspace prune failed", "error", err)
	} else if removed > 0 {
		observability.Info(ctx, "workspace prune completed", "removed", removed)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if removed, err := o.pruneWorkspaces(ctx); err != nil {
				observability.Warn(ctx, "workspace prune failed", "error", err)
			} else if removed > 0 {
				observability.Info(ctx, "workspace prune completed", "removed", removed)
			}
		}
	}
}

func (o *Orchestrator) ensureWorkspaceCloned(ctx context.Context, task *models.Task, agent *models.Agent) error {
	if o.repositories == nil {
		return fmt.Errorf("repositories lookup not configured")
	}
	if o.gitOps == nil {
		return fmt.Errorf("gitops client not configured")
	}
	repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
	if err != nil {
		return fmt.Errorf("list project repositories: %w", err)
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repository linked to project %s", task.ProjectID)
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
	var repo *models.Repository
	if task.RepositoryID != nil {
		for _, r := range repos {
			if r.ID == *task.RepositoryID {
				repo = &r
				break
			}
		}
		if repo == nil {
			return fmt.Errorf("repository %s not found in project", *task.RepositoryID)
		}
		
		localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
		gitDir := filepath.Join(localPath, ".git")
		
		workspaceExists := false
		if stat, err := os.Stat(gitDir); err == nil && stat.IsDir() {
			workspaceExists = true
		}

		if workspaceExists {
			if !hasSuccessfulCodeStep {
				if err := resetExistingWorkspace(ctx, localPath); err != nil {
					return fmt.Errorf("reset existing workspace: %w", err)
				}
			}
		} else {
			os.RemoveAll(localPath)
			if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
				return fmt.Errorf("create workspace parent dir: %w", err)
			}
			if _, err := o.gitOps.CloneForTask(ctx, repo.URL, repo.Branch, localPath); err != nil {
				return fmt.Errorf("clone repo: %w", err)
			}
			workspaceRestored = true
		}
	} else {
		localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
		
		workspaceExists := true
		for _, r := range repos {
			parts := strings.Split(r.URL, "/")
			repoName := parts[len(parts)-1]
			repoName = strings.TrimSuffix(repoName, ".git")
			subPath := filepath.Join(localPath, repoName)
			if stat, err := os.Stat(subPath); err != nil || !stat.IsDir() {
				workspaceExists = false
				break
			}
		}

		if !workspaceExists || !hasSuccessfulCodeStep {
			os.RemoveAll(localPath)
			if err := os.MkdirAll(localPath, 0o755); err != nil {
				return fmt.Errorf("create multi-workspace parent dir: %w", err)
			}

			for _, r := range repos {
				parts := strings.Split(r.URL, "/")
				repoName := parts[len(parts)-1]
				repoName = strings.TrimSuffix(repoName, ".git")
				subPath := filepath.Join(localPath, repoName)
				
				if _, err := o.gitOps.CloneForTask(ctx, r.URL, r.Branch, subPath); err != nil {
					return fmt.Errorf("clone multi-repo %s: %w", repoName, err)
				}
			}
			workspaceRestored = true
		}
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
							if errApply := o.applyPatch(ctx, task, agent, art.Step+"_restore", patchText); errApply != nil {
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

func (o *Orchestrator) cleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	if o.retention.Retention != 0 {
		return
	}
	if o.tasks != nil {
		task, err := o.tasks.GetByID(ctx, taskID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "record not found") {
				_ = o.removeWorkspace(taskID)
			}
			return
		}
		if task.Status == models.TaskStatusMerged || task.Status == models.TaskStatusFailed {
			if err := o.removeWorkspace(taskID); err != nil {
				observability.Warn(ctx, "workspace cleanup failed", "task_id", taskID, "error", err)
				return
			}
			observability.Info(ctx, "workspace cleaned after final state", "task_id", taskID)
		}
	} else {
		if err := o.removeWorkspace(taskID); err != nil {
			observability.Warn(ctx, "workspace cleanup failed", "task_id", taskID, "error", err)
			return
		}
		observability.Info(ctx, "workspace cleaned after final state", "task_id", taskID)
	}
}

func (o *Orchestrator) pruneWorkspaces(ctx context.Context) (int, error) {
	root := o.workspaceRoot
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-o.retention.Retention)
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			observability.Warn(ctx, "workspace prune stat failed", "name", entry.Name(), "error", err)
			continue
		}
		taskID := entry.Name()
		if o.tasks != nil {
			task, err := o.tasks.GetByID(ctx, taskID)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "record not found") {
					if err := o.removeWorkspace(taskID); err == nil {
						removed++
					}
				}
				continue
			}
			if task.Status == models.TaskStatusMerged || task.Status == models.TaskStatusFailed {
				if err := o.removeWorkspace(taskID); err != nil {
					observability.Warn(ctx, "workspace prune remove failed", "name", taskID, "error", err)
					continue
				}
				removed++
			}
		} else {
			if info.ModTime().Before(cutoff) {
				if err := o.removeWorkspace(entry.Name()); err != nil {
					observability.Warn(ctx, "workspace prune remove failed", "name", entry.Name(), "error", err)
					continue
				}
				removed++
			}
		}
	}
	return removed, nil
}

func (o *Orchestrator) removeWorkspace(taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task id is required")
	}
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

func (o *Orchestrator) StartLogPruner(ctx context.Context, retentionDays int, fileRoot string) {
	if retentionDays <= 0 || fileRoot == "" {
		return
	}
	interval := time.Hour
	if pruned, err := pruneLogFiles(ctx, retentionDays, fileRoot); err != nil {
		observability.Warn(ctx, "log files prune failed", "error", err)
	} else if pruned > 0 {
		observability.Info(ctx, "log files prune completed", "pruned", pruned)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if pruned, err := pruneLogFiles(ctx, retentionDays, fileRoot); err != nil {
				observability.Warn(ctx, "log files prune failed", "error", err)
			} else if pruned > 0 {
				observability.Info(ctx, "log files prune completed", "pruned", pruned)
			}
		}
	}
}

func pruneLogFiles(ctx context.Context, retentionDays int, fileRoot string) (int, error) {
	entries, err := os.ReadDir(fileRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	pruned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			observability.Warn(ctx, "log prune stat failed", "name", entry.Name(), "error", err)
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		filePath := filepath.Join(fileRoot, entry.Name())
		if err := os.Remove(filePath); err != nil {
			observability.Warn(ctx, "log prune remove failed", "path", filePath, "error", err)
			continue
		}
		pruned++
	}
	return pruned, nil
}
