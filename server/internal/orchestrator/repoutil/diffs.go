package repoutil

import (
	"context"

	orchestratorpatch "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (m *Manager) ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	runner := orchestratorpatch.Runner{
		WorkspaceRoot:            m.WorkspaceRoot,
		GetTaskRepoHostPath:      m.GetTaskRepoHostPath,
		HostWorktreePath:         m.HostWorktreePath,
		ContainerPathForHostPath: m.ContainerPathForHostPath,
		RunSandboxStepInWorktree: m.RunSandboxStepInWorktree,
	}
	return runner.ApplyPatch(ctx, task, agent, stepID, patchText, worktreeSuffix)
}

func (m *Manager) CaptureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	runner := orchestratorpatch.Runner{
		WorkspaceRoot:            m.WorkspaceRoot,
		GetTaskRepoHostPath:      m.GetTaskRepoHostPath,
		HostWorktreePath:         m.HostWorktreePath,
		ContainerPathForHostPath: m.ContainerPathForHostPath,
		GetDiff:                  m.SandboxGitGetDiff,
		GetWorkspaceDiff:         m.SandboxGitGetWorkspaceDiff,
	}
	return runner.CaptureWorkspaceDiff(ctx, task, agent, stepID, worktreeSuffix)
}

func (m *Manager) CapturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error) {
	runner := orchestratorpatch.Runner{
		WorkspaceRoot:            m.WorkspaceRoot,
		GetTaskRepoHostPath:      m.GetTaskRepoHostPath,
		HostWorktreePath:         m.HostWorktreePath,
		ContainerPathForHostPath: m.ContainerPathForHostPath,
		GetPRDiff:                m.SandboxGitGetPRDiff,
		LoadTaskWorkspace:        m.LoadTaskWorkspace,
	}
	if m.ListRepositories != nil {
		runner.ListRepositories = m.ListRepositories
	}
	return runner.CapturePRDiff(ctx, task, agent, baseBranch)
}

func (m *Manager) GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	runner := orchestratorpatch.Runner{
		ContainerPathForHostPath: m.ContainerPathForHostPath,
		SandboxGetChangedFiles:   m.SandboxGitGetChangedFiles,
		LoadTaskWorkspace:        m.LoadTaskWorkspace,
		GetRoleFromSuffix:        wkspace.GetRoleFromSuffix,
	}
	if m.ListRepositories != nil {
		runner.ListRepositories = m.ListRepositories
	}
	return runner.GetChangedFiles(ctx, task, agent, targetPath, worktreeSuffix)
}
