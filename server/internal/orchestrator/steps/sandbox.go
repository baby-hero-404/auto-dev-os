package steps

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func setupSandbox(
	ctx context.Context,
	task *models.Task,
	agent *models.Agent,
	worktree WorktreeManager,
	workspace WorkspaceLoader,
	prefix, role, suffix string,
) error {
	if worktree == nil {
		return nil
	}
	targetRepos, err := worktree.LoadTargetRepositories(ctx, task)
	if err != nil {
		return nil
	}
	var ws *models.TaskWorkspace
	if workspace != nil {
		ws, _ = workspace.LoadTaskWorkspace(ctx, task)
	}
	return worktree.SetupRoleWorktrees(ctx, task, agent, targetRepos, ws, prefix, role, suffix)
}

func commitSandbox(
	ctx context.Context,
	task *models.Task,
	agent *models.Agent,
	worktree WorktreeManager,
	workspace WorkspaceLoader,
	prefix, role, suffix string,
) error {
	if worktree == nil {
		return nil
	}
	targetRepos, err := worktree.LoadTargetRepositories(ctx, task)
	if err != nil {
		return nil
	}
	var ws *models.TaskWorkspace
	if workspace != nil {
		ws, _ = workspace.LoadTaskWorkspace(ctx, task)
	}
	return worktree.CommitRoleWorktrees(ctx, task, agent, targetRepos, ws, prefix, role, suffix)
}
