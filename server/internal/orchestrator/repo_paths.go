package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	orchestratorpatch "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
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
	return filepath.Join(localPath, "code", "repos", repoNameFromURL(repo.URL), "main")
}

func (o *Orchestrator) applyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	runner := orchestratorpatch.Runner{
		WorkspaceRoot:            o.workspaceRoot,
		GetTaskRepoHostPath:      o.getTaskRepoHostPath,
		HostWorktreePath:         o.hostWorktreePath,
		ContainerPathForHostPath: o.containerPathForHostPath,
		RunSandboxStepInWorktree: o.runSandboxStepInWorktree,
	}
	return runner.ApplyPatch(ctx, task, agent, stepID, patchText, worktreeSuffix)
}

func (o *Orchestrator) captureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	runner := orchestratorpatch.Runner{
		WorkspaceRoot:            o.workspaceRoot,
		GetTaskRepoHostPath:      o.getTaskRepoHostPath,
		HostWorktreePath:         o.hostWorktreePath,
		ContainerPathForHostPath: o.containerPathForHostPath,
		GetDiff:                  o.sandboxGit.GetDiff,
		GetWorkspaceDiff:         o.sandboxGit.GetWorkspaceDiff,
	}
	return runner.CaptureWorkspaceDiff(ctx, task, agent, stepID, worktreeSuffix)
}

func (o *Orchestrator) capturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error) {
	runner := orchestratorpatch.Runner{
		WorkspaceRoot:            o.workspaceRoot,
		GetTaskRepoHostPath:      o.getTaskRepoHostPath,
		HostWorktreePath:         o.hostWorktreePath,
		ContainerPathForHostPath: o.containerPathForHostPath,
		GetPRDiff:                o.sandboxGit.GetPRDiff,
	}
	return runner.CapturePRDiff(ctx, task, agent, baseBranch)
}

func (o *Orchestrator) getChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	runner := orchestratorpatch.Runner{
		ContainerPathForHostPath: o.containerPathForHostPath,
		SandboxGetChangedFiles:   o.sandboxGit.GetChangedFiles,
		LoadTaskWorkspace:        o.LoadTaskWorkspace,
		GetRoleFromSuffix:        getRoleFromSuffix,
	}
	if o.repositories != nil {
		runner.ListRepositories = o.repositories.ListByProjectID
	}
	return runner.GetChangedFiles(ctx, task, agent, targetPath, worktreeSuffix)
}

func (o *Orchestrator) hostWorktreePath(task *models.Task, repoPath string, worktreeSuffix string) string {
	if worktreeSuffix == "" {
		return repoPath
	}

	ctx := context.Background()
	rWS, err := o.FindRepoWorkspaceByPath(ctx, task, repoPath)
	if err != nil {
		clean := strings.TrimPrefix(worktreeSuffix, "-")
		clean = strings.TrimSuffix(clean, "-worktree")
		localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
		if task.RepositoryID != nil {
			return filepath.Join(localPath, clean)
		}
		if repoPath == localPath {
			return localPath
		}
		return repoPath + worktreeSuffix
	}

	role := getRoleFromSuffix(worktreeSuffix)
	ws := o.GetTaskWorkspace(task)

	if rWS.Paths.Worktrees == nil {
		rWS.Paths.Worktrees = make(map[string]string)
	}
	if rWS.Branches.Role == nil {
		rWS.Branches.Role = make(map[string]string)
	}

	if path, exists := rWS.Paths.Worktrees[role]; exists && path != "" {
		return filepath.Join(ws.Root, path)
	}

	relPath := filepath.Join("code", "repos", rWS.Name, "worktrees", role)
	rWS.Paths.Worktrees[role] = relPath
	rWS.Branches.Role[role] = fmt.Sprintf("feature/%s-%s", task.ID, role)

	if wsLoaded, errLoad := o.LoadTaskWorkspace(ctx, task); errLoad == nil {
		for i := range wsLoaded.Repos {
			if wsLoaded.Repos[i].RepoID == rWS.RepoID {
				wsLoaded.Repos[i] = *rWS
				break
			}
		}
		_ = o.SaveTaskWorkspaceMetadata(task, wsLoaded)
	}

	return filepath.Join(ws.Root, relPath)
}

func (o *Orchestrator) getTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	if task.RepositoryID == nil {
		return localPath, nil
	}
	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err == nil && ws != nil {
		for _, r := range ws.Repos {
			if r.RepoID == *task.RepositoryID {
				if r.Paths.Main == "" {
					return "", fmt.Errorf("task repository %s has empty main path in workspace metadata", *task.RepositoryID)
				}
				return filepath.Join(ws.Root, r.Paths.Main), nil
			}
		}
	}
	if o.repositories != nil {
		if repos, errList := o.repositories.ListByProjectID(ctx, task.ProjectID); errList == nil {
			for _, repo := range repos {
				if repo.ID == *task.RepositoryID {
					return o.repoHostPath(task, ws, repo), nil
				}
			}
		}
	}
	return "", fmt.Errorf("task repository %s not found in workspace metadata or project repositories", *task.RepositoryID)
}
