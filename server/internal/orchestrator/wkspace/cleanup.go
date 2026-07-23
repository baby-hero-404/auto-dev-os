package wkspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// CleanupWorkspaceAfterFinalState releases locks and prunes workspace repositories to save space.
func (m *Manager) CleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	m.ReleaseWorkspaceLock(taskID)

	if err := m.PartialCleanupWorkspace(ctx, taskID); err != nil {
		observability.Warn(ctx, "workspace partial cleanup failed", "task_id", taskID, "error", err)
	} else {
		observability.Info(ctx, "workspace partially cleaned after final state", "task_id", taskID)
	}
}

// PartialCleanupWorkspace removes all cloned repositories under code/repos/ while preserving diffs and metadata.
func (m *Manager) PartialCleanupWorkspace(ctx context.Context, taskID string) error {
	m.ReleaseWorkspaceLock(taskID)

	root := sandbox.WorkspacePath(m.WorkspaceRoot, taskID)
	wp := paths.NewOSWorkspacePaths(m.WorkspaceRoot)
	codeDir := wp.CodeRoot(taskID).String()

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
		wtParentDir := wp.RepoRoot(taskID, repoName).Child("worktrees").String()
		worktrees, err := os.ReadDir(wtParentDir)
		if err == nil {
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
					if diffErr == nil {
						statusClean := strings.TrimSpace(string(statusOut))
						fullDiffContent := []byte(fmt.Sprintf("=== Worktree Status ===\n%s\n\n=== Diffs ===\n%s", statusClean, string(diffOut)))

						diffDir := filepath.Join(root, "artifacts", "diffs")
						_ = os.MkdirAll(diffDir, 0o755)
						diffPath := filepath.Join(diffDir, fmt.Sprintf("cleanup-%s-%s.diff", repoName, role))
						_ = os.WriteFile(diffPath, fullDiffContent, 0o644)
					}
				}
			}
		}

		// Clean up worktrees to save space, but KEEP the main repository clone.
		repoMain := wp.RepoMain(taskID, repoName).String()
		if _, err := os.Stat(wtParentDir); err == nil && worktrees != nil {
			for _, wtEntry := range worktrees {
				if !wtEntry.IsDir() {
					continue
				}
				role := wtEntry.Name()
				wtAbs := filepath.Join(wtParentDir, role)

				// Remove worktree cleanly via Git
				rmCmd := exec.CommandContext(ctx, "git", "-C", repoMain, "worktree", "remove", "-f", wtAbs)
				if rmErr := rmCmd.Run(); rmErr != nil {
					// Fallback: just delete the directory. A failure here is
					// usually root-owned files written by the sandbox
					// container — surfacing it is the only way an operator
					// can spot workspaces leaking disk instead of silently
					// accumulating.
					if delErr := os.RemoveAll(wtAbs); delErr != nil {
						observability.Warn(ctx, "workspace worktree cleanup failed (possible root-owned files from sandbox)", "task_id", taskID, "path", wtAbs, "error", delErr)
					}
				}
			}
			// Delete the worktrees parent directory as it should be empty now
			if delErr := os.RemoveAll(wtParentDir); delErr != nil {
				observability.Warn(ctx, "workspace worktrees dir cleanup failed (possible root-owned files from sandbox)", "task_id", taskID, "path", wtParentDir, "error", delErr)
			}
		}
	}

	// Update metadata.json if it exists and can be loaded
	if m.Tasks != nil {
		if task, err := m.Tasks.GetByID(ctx, taskID); err == nil {
			if ws, errLoad := m.LoadTaskWorkspace(ctx, task); errLoad == nil {
				for i := range ws.Repos {
					ws.Repos[i].Paths.Worktrees = make(map[string]string)
					ws.Repos[i].Branches.Role = make(map[string]string)
				}
				_ = m.SaveTaskWorkspaceMetadata(task, ws)
			}
		}
	}

	return nil
}

// RemoveWorkspace deletes the entire workspace directory for a task.
func (m *Manager) RemoveWorkspace(taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task id is required")
	}
	m.ReleaseWorkspaceLock(taskID)

	// Finding 8: DB Checkpoint & Artifact Pruning
	if m.Workflows != nil {
		_ = m.Workflows.DeleteByTaskID(context.Background(), taskID)
	}
	if m.Artifacts != nil {
		_ = m.Artifacts.DeleteByTaskID(context.Background(), taskID)
	}

	// Clean up the task's log file (<LogFileRoot>/<taskID>.jsonl)
	if m.LogFileRoot != "" {
		logPath := filepath.Join(m.LogFileRoot, taskID+".jsonl")
		if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
			// Non-fatal: log file may have already been pruned by retention
			_ = err
		}
	}

	root := m.WorkspaceRoot
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
