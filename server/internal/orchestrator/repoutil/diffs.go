package repoutil

import (
	"context"
	"fmt"

	orchestratorpatch "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (m *Manager) getEngine(ctx context.Context) orchestratorpatch.PatchEngine {
	runner := &orchestratorpatch.Runner{
		WorkspaceRoot:            m.WorkspaceRoot,
		GetTaskRepoHostPath:      m.GetTaskRepoHostPath,
		HostWorktreePath:         m.HostWorktreePath,
		ContainerPathForHostPath: m.ContainerPathForHostPath,
		RunSandboxStepInWorktree: m.RunSandboxStepInWorktree,
		LoadTaskWorkspace:        m.LoadTaskWorkspace,
		GetRoleFromSuffix:        wkspace.GetRoleFromSuffix,
		UpdateTaskAnalysis:       m.UpdateTaskAnalysis,
		Log: func(ctx context.Context, taskID string, level string, message string) {
			if m.Log != nil {
				m.Log(ctx, taskID, nil, level, message)
			}
		},
	}
	if m.ListRepositories != nil {
		runner.ListRepositories = m.ListRepositories
	}

	strategy := "unified_diff"
	if prompts.UseSearchReplace(ctx) {
		strategy = "search_replace"
	}
	return orchestratorpatch.NewEngine(strategy, runner)
}

func (m *Manager) Validate(ctx context.Context, task *models.Task, patchData string, worktreeSuffix string) []error {
	engine := m.getEngine(ctx)

	// Actually, we can get repo path:
	basePath := ""
	if task.RepositoryID != nil {
		if repoPath, err := m.GetTaskRepoHostPath(ctx, task); err == nil {
			basePath = m.HostWorktreePath(task, repoPath, worktreeSuffix)
		}
	} else {
		// Just use a dummy or let engine figure it out if multi-repo
		basePath = m.WorkspaceRoot // fallback
	}

	var errs []error
	for _, vErr := range engine.Validate(ctx, patchData, basePath) {
		errs = append(errs, fmt.Errorf("%s: %s", vErr.Filepath, vErr.Reason))
	}
	return errs
}

func (m *Manager) ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	engine := m.getEngine(ctx)
	return engine.Apply(ctx, task, agent, stepID, patchText, worktreeSuffix)
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
