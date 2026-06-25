package repoutil

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func RepoNameFromURL(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	repoName := parts[len(parts)-1]
	return strings.TrimSuffix(repoName, ".git")
}

func (m *Manager) RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string {
	localPath := sandbox.WorkspacePath(m.WorkspaceRoot, task.ID)
	if ws != nil {
		for i := range ws.Repos {
			if ws.Repos[i].RepoID == repo.ID {
				return filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
			}
		}
	}
	if task.RepositoryID == nil {
		return filepath.Join(localPath, RepoNameFromURL(repo.URL))
	}
	return filepath.Join(localPath, "code", "repos", RepoNameFromURL(repo.URL), "main")
}

func (m *Manager) GetTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error) {
	localPath := sandbox.WorkspacePath(m.WorkspaceRoot, task.ID)
	if task.RepositoryID == nil {
		return localPath, nil
	}
	ws, err := m.LoadTaskWorkspace(ctx, task)
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
	if m.ListRepositories != nil {
		if repos, errList := m.ListRepositories(ctx, task.ProjectID); errList == nil {
			for _, repo := range repos {
				if repo.ID == *task.RepositoryID {
					return m.RepoHostPath(task, ws, repo), nil
				}
			}
		}
	}
	return "", fmt.Errorf("task repository %s not found in workspace metadata or project repositories", *task.RepositoryID)
}

func (m *Manager) HostWorktreePath(task *models.Task, repoPath string, worktreeSuffix string) string {
	if worktreeSuffix == "" {
		return repoPath
	}

	ctx := context.Background()
	rWS, err := m.FindRepoWorkspaceByPath(ctx, task, repoPath)
	if err != nil {
		clean := strings.TrimPrefix(worktreeSuffix, "-")
		clean = strings.TrimSuffix(clean, "-worktree")
		localPath := sandbox.WorkspacePath(m.WorkspaceRoot, task.ID)
		if task.RepositoryID != nil {
			return filepath.Join(localPath, clean)
		}
		if repoPath == localPath {
			return localPath
		}
		return repoPath + worktreeSuffix
	}

	role := wkspace.GetRoleFromSuffix(worktreeSuffix)
	ws := m.GetTaskWorkspace(task)

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

	if wsLoaded, errLoad := m.LoadTaskWorkspace(ctx, task); errLoad == nil {
		for i := range wsLoaded.Repos {
			if wsLoaded.Repos[i].RepoID == rWS.RepoID {
				wsLoaded.Repos[i] = *rWS
				break
			}
		}
		_ = m.SaveTaskWorkspaceMetadata(task, wsLoaded)
	}

	return filepath.Join(ws.Root, relPath)
}
