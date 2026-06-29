package repoutil

import (
	"context"
	"fmt"

	orchestratorpatch "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (m *Manager) getEngine() orchestratorpatch.PatchEngine {
	runner := &orchestratorpatch.Runner{
		WorkspaceRoot:            m.WorkspaceRoot,
		GetTaskRepoHostPath:      m.GetTaskRepoHostPath,
		HostWorktreePath:         m.HostWorktreePath,
		ContainerPathForHostPath: m.ContainerPathForHostPath,
		RunSandboxStepInWorktree: m.RunSandboxStepInWorktree,
		LoadTaskWorkspace:        m.LoadTaskWorkspace,
		GetRoleFromSuffix:        wkspace.GetRoleFromSuffix,
	}
	if m.ListRepositories != nil {
		runner.ListRepositories = m.ListRepositories
	}
	// TODO: dynamically select strategy if config supports it, otherwise default to search_replace or unified_diff.
	// For Phase 12, we can return search_replace to test it, but plan says "opt-in via a configuration flag".
	// Let's use "search_replace" if preferred, else "unified_diff". We'll hardcode "unified_diff" for now as default.
	return orchestratorpatch.NewEngine("unified_diff", runner)
}

func (m *Manager) Validate(ctx context.Context, task *models.Task, patchData string, worktreeSuffix string) []error {
	engine := m.getEngine()

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
	for _, vErr := range engine.Validate(patchData, basePath) {
		errs = append(errs, fmt.Errorf("%s: %s", vErr.Filepath, vErr.Reason))
	}
	return errs
}

func (m *Manager) ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	engine := m.getEngine()
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
