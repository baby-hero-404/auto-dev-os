package repoutil

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func RepoNameFromURL(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	repoName := parts[len(parts)-1]
	return strings.TrimSuffix(repoName, ".git")
}

func (m *Manager) RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string {
	if ws != nil {
		for i := range ws.Repos {
			if ws.Repos[i].RepoID == repo.ID {
				return filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
			}
		}
	}
	wp := paths.NewOSWorkspacePaths(m.WorkspaceRoot)
	return wp.RepoMain(task.ID, RepoNameFromURL(repo.URL)).String()
}

func (m *Manager) GetTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error) {
	localPath := sandbox.WorkspacePath(m.WorkspaceRoot, task.ID)
	ws, err := m.LoadTaskWorkspace(ctx, task)
	if task.RepositoryID == nil {
		// RepositoryID is an optional CreateTaskInput field (see
		// TaskService.Create) — a task can legitimately have a repo checked
		// out (single-repo project) without one ever being assigned. Falling
		// straight back to localPath here made every CLI-mode step (cwd,
		// docs/openspecs read-back) operate on the bare task workspace root
		// instead of the actual repo checkout, silently diverging from
		// where the CLI agent itself resolves and writes files (it explores
		// and finds the real repo regardless), which is what produced
		// "missing required spec file(s)" even though the files existed on
		// disk. Only resolve automatically when there's exactly one checked
		// -out repo; with more than one, resolveContextRepoPaths' prefix-
		// disambiguation shows this function has no way to pick one, so keep
		// the existing bare-localPath behavior for that ambiguous case.
		if err == nil && ws != nil {
			var onlyRepo *models.RepoWorkspace
			count := 0
			for i := range ws.Repos {
				if ws.Repos[i].Paths.Main == "" {
					continue
				}
				count++
				onlyRepo = &ws.Repos[i]
			}
			if count == 1 {
				return filepath.Join(ws.Root, onlyRepo.Paths.Main), nil
			}
		}
		return localPath, nil
	}
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
	if m.Log != nil {
		m.Log(ctx, task.ID, nil, "error", fmt.Sprintf("repoutil: GetTaskRepoHostPath -> repository %s not found in workspace metadata or project repositories", *task.RepositoryID))
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

	wp := paths.NewOSWorkspacePaths(m.WorkspaceRoot)
	relPath := wp.RepoWorktreeRelative(rWS.Name, role)
	rWS.Paths.Worktrees[role] = relPath
	roleSuffix := role
	if role == "backend" {
		roleSuffix = "be"
	} else if role == "frontend" {
		roleSuffix = "fe"
	}
	rWS.Branches.Role[role] = paths.DeriveRoleBranchName(task.ID, task.Title, roleSuffix)

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
