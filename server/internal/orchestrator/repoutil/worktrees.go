package repoutil

import (
	"context"
	"fmt"
	"strings"

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
	userName := "AutoCodeOS Agent"
	userEmail := "agent@autocode.os"
	if agent != nil {
		if agent.Name != "" {
			userName = agent.Name
		}
		if agent.ID != "" {
			userEmail = agent.ID + "@autocode.os"
		}
	}

	for _, repo := range repos {
		localPath := m.RepoHostPath(task, ws, repo)
		worktreePath := m.HostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := m.ContainerPathForHostPath(task, worktreePath, "")
		script := fmt.Sprintf(`set -e
git -C %[1]s config user.name %[3]s
git -C %[1]s config user.email %[4]s
if [ -n "$(git -C %[1]s status --porcelain)" ]; then
    git -C %[1]s add .
    git -C %[1]s commit -m %[2]s
fi`,
			paths.QuoteShellArg(containerWorktreePath),
			paths.QuoteShellArg(commitMsg),
			paths.QuoteShellArg(userName),
			paths.QuoteShellArg(userEmail),
		)
		if _, err := m.RunSandboxStepInWorktree(ctx, task, agent, "commit_"+roleName, script, worktreeSuffix); err != nil {
			return fmt.Errorf("failed to commit changes for repo %s: %w", repo.URL, err)
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

		commitMsg := fmt.Sprintf("chore(auto-code-os): checkpoint %s", stepID)

		script := fmt.Sprintf(`set -e
git -C %[1]s add -u
git -C %[1]s status --porcelain | grep '^??' | cut -c 4- | while read -r file; do
  if [ -n "$file" ]; then
    case "$file" in
      *.go|*.ts|*.tsx|*.js|*.jsx|*.css|*.json|*.sql|*.yaml|*.yml|*.toml|*.md|*go.mod|*go.sum)
        git -C %[1]s add "$file"
        ;;
    esac
  fi
done
git -C %[1]s commit -m %[2]s --allow-empty
git -C %[1]s rev-parse HEAD`, paths.QuoteShellArg(containerWorktreePath), paths.QuoteShellArg(commitMsg))

		res, err := m.RunSandboxStepInWorktree(ctx, task, agent, "checkpoint_"+stepID, script, worktreeSuffix)
		if err != nil {
			return "", fmt.Errorf("failed to create checkpoint commit for repo %s: %w", repo.URL, err)
		}
		if stdout, ok := res["stdout"].(string); ok {
			lastCommitHash = strings.TrimSpace(stdout)
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

		script := fmt.Sprintf(`set -e
git -C %[1]s checkout %[2]s
git -C %[1]s reset --hard HEAD
git -C %[1]s clean -fd`, paths.QuoteShellArg(containerWorktreePath), paths.QuoteShellArg(commitHash))

		if _, err := m.RunSandboxStepInWorktree(ctx, task, agent, "restore_checkpoint", script, worktreeSuffix); err != nil {
			return fmt.Errorf("failed to restore checkpoint to %s for repo %s: %w", commitHash, repo.URL, err)
		}
	}
	return nil
}
