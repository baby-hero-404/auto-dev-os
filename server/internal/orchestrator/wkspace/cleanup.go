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
// The lock release is synchronous (cheap, and callers may rely on it before
// returning), but the actual disk cleanup (git status/diff + RemoveAll of a
// full repo clone) runs in the background: several call sites invoke this
// directly from an HTTP request path (PR approve, PR-merged webhook), and
// blocking those on cleanup would add unrelated latency to the response.
func (m *Manager) CleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	m.ReleaseWorkspaceLock(ctx, taskID)

	// A decomposition child shares its parent's workspace (models.
	// WorkspaceOwnerID) with the rest of the family. Reaching a final state
	// itself only means *this* child is done, not that the whole family is
	// — a not-yet-started sibling still needs that shared clone. Skip disk
	// cleanup here and let the parent's own terminal transition
	// (completeDecomposedParent, decomposition.go) trigger it once every
	// child has actually finished.
	if m.Tasks != nil {
		if task, err := m.Tasks.GetByID(ctx, taskID); err == nil && task.ParentTaskID != nil {
			return
		}
	}

	go func() {
		bgCtx := context.WithoutCancel(ctx)
		if err := m.PartialCleanupWorkspace(bgCtx, taskID); err != nil {
			observability.Warn(bgCtx, "workspace partial cleanup failed", "task_id", taskID, "error", err)
		} else {
			observability.Info(bgCtx, "workspace partially cleaned after final state", "task_id", taskID)
		}
	}()
}

// PartialCleanupWorkspace removes all cloned repositories under code/repos/
// (main checkout and worktrees alike) while preserving logs, specs, and
// captured diffs/metadata, which live outside code/repos/ and are untouched.
func (m *Manager) PartialCleanupWorkspace(ctx context.Context, taskID string) error {
	m.ReleaseWorkspaceLock(ctx, taskID)

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

	// Last-chance safety net for tasks whose cli_spec ran before the specs/
	// dir mirroring existed: if specs/ is still empty, grab the OpenSpec
	// bundle straight out of the repo checkout before it's deleted below,
	// so older in-flight tasks don't permanently lose spec access on their
	// first merge-triggered cleanup.
	specsDir := wp.SpecsDir(taskID).String()
	specsDirEmpty := true
	if entries, statErr := os.ReadDir(specsDir); statErr == nil && len(entries) > 0 {
		specsDirEmpty = false
	}
	var specSlug string
	if specsDirEmpty && m.Tasks != nil {
		if task, tErr := m.Tasks.GetByID(ctx, taskID); tErr == nil {
			specSlug = paths.DeriveTaskSlug(task.ID, task.Title)
		}
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

		// Capture uncommitted changes in the main checkout itself before
		// deleting it, same safety net as the worktree loop above.
		if stat, statErr := os.Stat(repoMain); statErr == nil && stat.IsDir() {
			statusCmd := exec.CommandContext(ctx, "git", "-C", repoMain, "status", "--porcelain")
			statusOut, statusErr := statusCmd.CombinedOutput()
			if statusErr == nil && len(strings.TrimSpace(string(statusOut))) > 0 {
				diffCmd := exec.CommandContext(ctx, "git", "-C", repoMain, "diff", "HEAD")
				diffOut, diffErr := diffCmd.CombinedOutput()
				if diffErr == nil {
					statusClean := strings.TrimSpace(string(statusOut))
					fullDiffContent := []byte(fmt.Sprintf("=== Main Checkout Status ===\n%s\n\n=== Diffs ===\n%s", statusClean, string(diffOut)))

					diffDir := filepath.Join(root, "artifacts", "diffs")
					_ = os.MkdirAll(diffDir, 0o755)
					diffPath := filepath.Join(diffDir, fmt.Sprintf("cleanup-%s-main.diff", repoName))
					_ = os.WriteFile(diffPath, fullDiffContent, 0o644)
				}
			}
		}

		if specsDirEmpty && specSlug != "" {
			srcDir := filepath.Join(repoMain, "docs", "openspecs", specSlug)
			if stat, statErr := os.Stat(srcDir); statErr == nil && stat.IsDir() {
				if mkErr := os.MkdirAll(specsDir, 0o755); mkErr == nil {
					cpCmd := exec.CommandContext(ctx, "cp", "-r", srcDir+"/.", specsDir+"/")
					if cpErr := cpCmd.Run(); cpErr != nil {
						observability.Warn(ctx, "fallback spec copy to workspace specs dir failed", "task_id", taskID, "path", srcDir, "error", cpErr)
					} else {
						specsDirEmpty = false
					}
				}
			}
		}

		// Remove the entire repo clone (main checkout included) to actually
		// reclaim disk space. Logs, specs, and captured diffs/metadata live
		// outside code/repos/ and are unaffected.
		repoDir := wp.RepoRoot(taskID, repoName).String()
		if delErr := os.RemoveAll(repoDir); delErr != nil {
			observability.Warn(ctx, "workspace repo clone cleanup failed (possible root-owned files from sandbox)", "task_id", taskID, "path", repoDir, "error", delErr)
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
	m.ReleaseWorkspaceLock(context.Background(), taskID)

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
