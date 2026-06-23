package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func repoNameFromURL(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	repoName := parts[len(parts)-1]
	return strings.TrimSuffix(repoName, ".git")
}

func targetRepositoriesForTask(task *models.Task, repos []models.Repository) []models.Repository {
	if task.RepositoryID == nil {
		return repos
	}
	for _, repo := range repos {
		if repo.ID == *task.RepositoryID {
			return []models.Repository{repo}
		}
	}
	return nil
}

func (o *Orchestrator) loadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error) {
	repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
	if err != nil {
		return nil, err
	}
	targetRepos := targetRepositoriesForTask(task, repos)
	if task.RepositoryID != nil && len(targetRepos) == 0 {
		return nil, fmt.Errorf("task repository %s not found", *task.RepositoryID)
	}
	return targetRepos, nil
}

func (o *Orchestrator) repoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	if ws != nil {
		for i := range ws.Repos {
			if ws.Repos[i].RepoID == repo.ID {
				return filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
			}
		}
	}
	if task.RepositoryID == nil {
		return filepath.Join(localPath, repoNameFromURL(repo.URL))
	}
	return localPath
}

func (o *Orchestrator) setupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace) {
	integrationBranch := fmt.Sprintf("feature/%s", task.ID)
	beBranch := fmt.Sprintf("feature/%s-be", task.ID)
	feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

	for _, repo := range repos {
		localPath := o.repoHostPath(task, ws, repo)
		containerLocalPath := o.containerPathForHostPath(task, localPath, "")
		script := fmt.Sprintf(`
set -e
git -C %[1]s show-ref --verify --quiet refs/heads/%[2]s || git -C %[1]s branch %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[1]s branch %[3]s %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[1]s branch %[4]s %[2]s
`, quoteShellArg(containerLocalPath), quoteShellArg(integrationBranch), quoteShellArg(beBranch), quoteShellArg(feBranch))

		if _, err := o.runSandboxStep(ctx, task, agent, "create_role_branches", script); err != nil {
			o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to create role branches for %s: %v", repo.URL, err))
		}
	}
}

func (o *Orchestrator) setupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	roleBranch := fmt.Sprintf("feature/%s-%s", task.ID, roleName)
	integrationBranch := fmt.Sprintf("feature/%s", task.ID)

	for _, repo := range repos {
		localPath := o.repoHostPath(task, ws, repo)
		worktreePath := o.hostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := o.containerPathForHostPath(task, worktreePath, "")
		containerLocalPath := o.containerPathForHostPath(task, localPath, "")
		script := fmt.Sprintf(`
set -e
git -C %[2]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[2]s branch %[4]s
git -C %[2]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[2]s branch %[3]s %[4]s
if [ -d %[1]s ] && grep -q '^gitdir:' %[1]s/.git 2>/dev/null; then
	echo 'worktree valid'
else
	rm -rf %[1]s
	git -C %[2]s worktree add %[1]s %[3]s
fi
`,
			quoteShellArg(containerWorktreePath),
			quoteShellArg(containerLocalPath),
			quoteShellArg(roleBranch),
			quoteShellArg(integrationBranch),
		)
		if _, err := o.runSandboxStep(ctx, task, agent, "worktree_"+roleName, script); err != nil {
			return fmt.Errorf("failed to setup %s worktree for repo %s: %w", roleLabel, repo.URL, err)
		}
	}
	return nil
}

func (o *Orchestrator) commitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) {
	commitMsg := fmt.Sprintf("AutoCodeOS [%s]: %s", roleLabel, task.Title)
	for _, repo := range repos {
		localPath := o.repoHostPath(task, ws, repo)
		worktreePath := o.hostWorktreePath(task, localPath, worktreeSuffix)
		containerWorktreePath := o.containerPathForHostPath(task, worktreePath, worktreeSuffix)
		script := fmt.Sprintf("git -C %[1]s add . && git -C %[1]s commit -m %[2]s || true",
			quoteShellArg(containerWorktreePath),
			quoteShellArg(commitMsg),
		)
		_, _ = o.runSandboxStepInWorktree(ctx, task, agent, "commit_"+roleName, script, worktreeSuffix)
	}
}
