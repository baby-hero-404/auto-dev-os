package repoutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func (m *Manager) SetupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
	integrationBranch := fmt.Sprintf("feature/%s", task.ID)
	beBranch := fmt.Sprintf("feature/%s-be", task.ID)
	feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

	for _, repo := range repos {
		localPath := m.RepoHostPath(task, ws, repo)
		containerLocalPath := m.ContainerPathForHostPath(task, localPath, "")
		var script string
		if skipFE {
			script = fmt.Sprintf(`
set -e
git -C %[1]s show-ref --verify --quiet refs/heads/%[2]s || git -C %[1]s branch %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[1]s branch %[3]s %[2]s
`, paths.QuoteShellArg(containerLocalPath), paths.QuoteShellArg(integrationBranch), paths.QuoteShellArg(beBranch))
		} else {
			script = fmt.Sprintf(`
set -e
git -C %[1]s show-ref --verify --quiet refs/heads/%[2]s || git -C %[1]s branch %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[1]s branch %[3]s %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[1]s branch %[4]s %[2]s
`, paths.QuoteShellArg(containerLocalPath), paths.QuoteShellArg(integrationBranch), paths.QuoteShellArg(beBranch), paths.QuoteShellArg(feBranch))
		}

		if _, err := m.RunSandboxStep(ctx, task, agent, "create_role_branches", script); err != nil {
			m.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to create role branches for %s: %v", repo.URL, err))
		}
	}
}

func (m *Manager) SetupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	roleBranch := fmt.Sprintf("feature/%s-%s", task.ID, roleName)
	integrationBranch := fmt.Sprintf("feature/%s", task.ID)

	for _, repo := range repos {
		localPath := m.RepoHostPath(task, ws, repo)
		worktreePath := m.HostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := m.ContainerPathForHostPath(task, worktreePath, "")
		containerLocalPath := m.ContainerPathForHostPath(task, localPath, "")
		script := fmt.Sprintf(`
set -e
exec 9> %[2]s/.git/worktree_setup.lock
flock -w 60 9
git -C %[2]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[2]s branch %[4]s
git -C %[2]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[2]s branch %[3]s %[4]s
if [ -d %[1]s ] && grep -q '^gitdir:' %[1]s/.git 2>/dev/null; then
	echo 'worktree valid'
else
	rm -r -f %[1]s
	git -C %[2]s worktree prune
	git -C %[2]s worktree add %[1]s %[3]s
fi
flock -u 9
`,
			paths.QuoteShellArg(containerWorktreePath),
			paths.QuoteShellArg(containerLocalPath),
			paths.QuoteShellArg(roleBranch),
			paths.QuoteShellArg(integrationBranch),
		)
		if _, err := m.RunSandboxStep(ctx, task, agent, "worktree_"+roleName, script); err != nil {
			return fmt.Errorf("failed to setup %s worktree for repo %s: %w", roleLabel, repo.URL, err)
		}
	}
	return nil
}

func (m *Manager) CommitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	commitMsg := fmt.Sprintf("AutoCodeOS [%s]: %s", roleLabel, task.Title)
	for _, repo := range repos {
		localPath := m.RepoHostPath(task, ws, repo)
		worktreePath := m.HostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := m.ContainerPathForHostPath(task, worktreePath, "")
		identityScript := m.gitIdentityScript(containerWorktreePath, agent)
		script := fmt.Sprintf(`set -e
%[3]s
if [ -n "$(git -C %[1]s status --porcelain)" ]; then
    git -C %[1]s add .
    git -C %[1]s commit -m %[2]s
fi`,
			paths.QuoteShellArg(containerWorktreePath),
			paths.QuoteShellArg(commitMsg),
			identityScript,
		)
		// commitSandbox failures are frequently transient (lock/IO contention in the sandbox);
		// retry a few times before surfacing an error, since a failure here after a successful
		// runPatchRetryLoop leaves the applied patch uncommitted and exposed to REQ-M04 data loss.
		const maxCommitAttempts = 3
		var lastErr error
		for attempt := 1; attempt <= maxCommitAttempts; attempt++ {
			if _, err := m.RunSandboxStepInWorktree(ctx, task, agent, "commit_"+roleName, script, worktreeSuffix); err != nil {
				lastErr = err
				if attempt < maxCommitAttempts {
					time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				}
				continue
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			return fmt.Errorf("failed to commit changes for repo %s after %d attempts: %w", repo.URL, maxCommitAttempts, lastErr)
		}
	}
	return nil
}

func (m *Manager) ResetRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, worktreeSuffix string) error {
	targetRepos, err := m.LoadTargetRepositories(ctx, task)
	if err != nil {
		return err
	}
	ws := m.GetTaskWorkspace(task)

	for _, repo := range targetRepos {
		localPath := m.RepoHostPath(task, ws, repo)
		worktreePath := m.HostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := m.ContainerPathForHostPath(task, worktreePath, "")

		script := fmt.Sprintf(`set -e
git -C %[1]s reset --hard HEAD
git -C %[1]s clean -fd`, paths.QuoteShellArg(containerWorktreePath))

		if _, err := m.RunSandboxStepInWorktree(ctx, task, agent, "reset_worktree", script, worktreeSuffix); err != nil {
			return fmt.Errorf("failed to reset worktree for repo %s: %w", repo.URL, err)
		}
	}
	return nil
}

func (m *Manager) CreateGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	targetRepos, err := m.LoadTargetRepositories(ctx, task)
	if err != nil {
		return "", err
	}
	ws := m.GetTaskWorkspace(task)

	var lastCommitHash string
	for _, repo := range targetRepos {
		localPath := m.RepoHostPath(task, ws, repo)
		worktreePath := m.HostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := m.ContainerPathForHostPath(task, worktreePath, "")

		identityScript := m.gitIdentityScript(containerWorktreePath, agent)
		commitMsg := fmt.Sprintf("chore(auto-code-os): checkpoint %s", stepID)

		// `git add -A` (not a hand-rolled extension whitelist) so newly created files are
		// captured even when they live inside a brand-new directory: `git status --porcelain`
		// reports such a directory as a single collapsed "?? dir/" line, which never matches a
		// per-extension case pattern, so a whitelist-based add loop silently drops the whole
		// directory. A later RestoreGitCheckpoint then runs `git clean -fd`, which permanently
		// deletes that never-committed, still-untracked work. `-A` mirrors the same-file rescue
		// snapshot in RestoreGitCheckpoint below, which already stages this way successfully.
		script := fmt.Sprintf(`set -e
%[3]s
git -C %[1]s add -A
staged_count=$(git -C %[1]s diff --cached --name-only | wc -l | tr -d ' ')
git -C %[1]s commit -q -m %[2]s --allow-empty
echo "STAGED_COUNT=$staged_count"
git -C %[1]s rev-parse HEAD`, paths.QuoteShellArg(containerWorktreePath), paths.QuoteShellArg(commitMsg), identityScript)

		res, err := m.RunSandboxStepInWorktree(ctx, task, agent, "checkpoint_"+stepID, script, worktreeSuffix)
		if err != nil {
			return "", fmt.Errorf("failed to create checkpoint commit for repo %s: %w", repo.URL, err)
		}
		if stdout, ok := res["stdout"].(string); ok {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) > 0 {
				lastCommitHash = strings.TrimSpace(lines[len(lines)-1])
			}
			for _, line := range lines {
				if count, cut := strings.CutPrefix(strings.TrimSpace(line), "STAGED_COUNT="); cut && count == "0" && m.Log != nil {
					m.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("checkpoint %s for repo %s staged 0 files — commit is empty, nothing from this step was captured", stepID, repo.URL))
				}
			}
		}
	}
	return lastCommitHash, nil
}

func (m *Manager) RestoreGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, commitHash string, worktreeSuffix string) error {
	targetRepos, err := m.LoadTargetRepositories(ctx, task)
	if err != nil {
		return err
	}
	ws := m.GetTaskWorkspace(task)

	for _, repo := range targetRepos {
		localPath := m.RepoHostPath(task, ws, repo)
		worktreePath := m.HostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := m.ContainerPathForHostPath(task, worktreePath, "")
		containerLocalPath := m.ContainerPathForHostPath(task, localPath, "")

		var roleName string
		switch worktreeSuffix {
		case models.WorktreeSuffixBackend:
			roleName = models.RoleBackend
		case models.WorktreeSuffixFrontend:
			roleName = models.RoleFrontend
		}

		// If a prior step's patch was applied but never committed (e.g. commitSandbox failed -
		// REQ-M04), the dirty worktree state below is about to be discarded by checkout/reset/clean.
		// Snapshot it onto a throwaway rescue branch first so the work is recoverable from git
		// history rather than permanently lost, even though the active worktree still resets cleanly.
		identityScript := m.gitIdentityScript(containerWorktreePath, agent)
		rescueBranch := fmt.Sprintf("rescue/%s-%d", task.ID, time.Now().UnixNano())
		// If the worktree's current HEAD already contains commitHash (e.g. a partial/salvaged
		// commit made further progress since the checkpoint being restored to), restoring would
		// silently discard that progress even though it's strictly ahead. Restore-to-checkpoint
		// runs unconditionally on every job resume, including internal review-fix loop-backs
		// where the worktree is already exactly where the previous attempt left it — so this
		// idempotency check (skip when HEAD is already at-or-past commitHash) is load-bearing,
		// not just an optimization.
		script := fmt.Sprintf(`set -e
%[4]s
if [ ! -d %[1]s ]; then
  echo "Recreating missing worktree directory..."
  roleBranch="feature/%[5]s-%[6]s"
  git -C %[7]s show-ref --verify --quiet refs/heads/$roleBranch || git -C %[7]s branch $roleBranch %[2]s
  git -C %[7]s worktree prune
  git -C %[7]s worktree add %[1]s $roleBranch
fi

if git -C %[1]s merge-base --is-ancestor %[2]s HEAD 2>/dev/null; then
  echo "SKIP_RESTORE: worktree already at or ahead of checkpoint %[2]s"
else
  if [ -n "$(git -C %[1]s status --porcelain)" ]; then
    git -C %[1]s add -A
    git -C %[1]s commit -m "AutoCodeOS: rescue snapshot before checkpoint restore" -q --allow-empty
    git -C %[1]s branch %[3]s
    git -C %[1]s reset --hard HEAD~1
  fi
  roleBranch="feature/%[5]s-%[6]s"
  git -C %[1]s checkout $roleBranch
  git -C %[1]s reset --hard %[2]s
  git -C %[1]s clean -fd
fi`, paths.QuoteShellArg(containerWorktreePath), paths.QuoteShellArg(commitHash), paths.QuoteShellArg(rescueBranch), identityScript, task.ID, roleName, paths.QuoteShellArg(containerLocalPath))

		if _, err := m.RunSandboxStepInWorktree(ctx, task, agent, "restore_checkpoint", script, worktreeSuffix); err != nil {
			return fmt.Errorf("failed to restore checkpoint to %s for repo %s: %w", commitHash, repo.URL, err)
		}
	}
	return nil
}

func (m *Manager) gitIdentityScript(containerPath string, agent *models.Agent) string {
	userName := m.DefaultAgentName
	userEmail := m.DefaultAgentEmail
	if agent != nil {
		if agent.Name != "" {
			userName = agent.Name
		}
		if agent.ID != "" {
			userEmail = agent.ID + "@autocode.os"
		}
	}
	return fmt.Sprintf(`git -C %[1]s config user.name %[2]s
git -C %[1]s config user.email %[3]s`,
		paths.QuoteShellArg(containerPath),
		paths.QuoteShellArg(userName),
		paths.QuoteShellArg(userEmail),
	)
}
