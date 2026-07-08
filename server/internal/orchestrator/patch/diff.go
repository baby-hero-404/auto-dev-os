package patch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func (r *Runner) CaptureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)
	targetPath := r.HostWorktreePath(task, localPath, worktreeSuffix)
	containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")

	if task.RepositoryID != nil {
		repoHostPath, err := r.GetTaskRepoHostPath(ctx, task)
		if err != nil {
			return "", err
		}
		targetPath = r.HostWorktreePath(task, repoHostPath, worktreeSuffix)
		containerTargetPath = r.ContainerPathForHostPath(task, targetPath, "")

		return r.GetDiff(ctx, task, agent, containerTargetPath)
	}

	// Multi-repo diff
	return r.GetWorkspaceDiff(ctx, task, agent, containerTargetPath, worktreeSuffix)
}

func (r *Runner) CapturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error) {
	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)
	targetPath := localPath
	containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")

	var ws *models.TaskWorkspace
	var errWS error
	if r.LoadTaskWorkspace != nil {
		ws, errWS = r.LoadTaskWorkspace(ctx, task)
	}

	if task.RepositoryID != nil {
		repoHostPath, err := r.GetTaskRepoHostPath(ctx, task)
		if err != nil {
			return "", err
		}
		targetPath = r.HostWorktreePath(task, repoHostPath, "")
		containerTargetPath = r.ContainerPathForHostPath(task, targetPath, "")

		resolvedBase := baseBranch
		if errWS == nil && ws != nil {
			for _, rWS := range ws.Repos {
				if rWS.RepoID == *task.RepositoryID && rWS.DefaultBranch != "" {
					resolvedBase = rWS.DefaultBranch
					break
				}
			}
		}

		return r.GetPRDiff(ctx, task, agent, containerTargetPath, resolvedBase)
	}

	if r.ListRepositories == nil {
		return "", fmt.Errorf("multi-repo PR diff requires repository listing")
	}
	repos, err := r.ListRepositories(ctx, task.ProjectID)
	if err != nil {
		return "", err
	}

	var diffOut []string
	for _, repo := range repos {
		repoHostPath := ""
		resolvedBase := baseBranch
		if errWS == nil && ws != nil {
			for _, rWS := range ws.Repos {
				if rWS.RepoID == repo.ID {
					repoHostPath = filepath.Join(ws.Root, rWS.Paths.Main)
					if rWS.DefaultBranch != "" {
						resolvedBase = rWS.DefaultBranch
					}
					break
				}
			}
		}
		if repoHostPath == "" {
			repoName := repoNameFromURL(repo.URL)
			repoHostPath = paths.NewOSWorkspacePaths(r.WorkspaceRoot).RepoMain(task.ID, repoName, "").String()
			if stat, statErr := os.Stat(repoHostPath); statErr != nil || !stat.IsDir() {
				repoHostPath = filepath.Join(localPath, repoName)
			}
		}
		containerRepoPath := r.ContainerPathForHostPath(task, repoHostPath, "")
		repoDiff, diffErr := r.GetPRDiff(ctx, task, agent, containerRepoPath, resolvedBase)
		if diffErr != nil {
			return "", fmt.Errorf("capture PR diff for repo %s: %w", repo.URL, diffErr)
		}
		if strings.TrimSpace(repoDiff) != "" {
			diffOut = append(diffOut, fmt.Sprintf("--- Repository: %s\n%s", repoNameFromURL(repo.URL), repoDiff))
		}
	}
	return strings.Join(diffOut, "\n"), nil
}

func (r *Runner) GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	var repos []models.Repository
	var err error
	if r.ListRepositories != nil {
		repos, err = r.ListRepositories(ctx, task.ProjectID)
	}
	if r.ListRepositories == nil || err != nil || len(repos) == 0 {
		containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")
		return r.SandboxGetChangedFiles(ctx, task, agent, containerTargetPath)
	}

	var targetRepos []models.Repository
	if task.RepositoryID != nil {
		for _, repo := range repos {
			if repo.ID == *task.RepositoryID {
				targetRepos = append(targetRepos, repo)
				break
			}
		}
	} else {
		targetRepos = repos
	}

	ws, errWS := r.LoadTaskWorkspace(ctx, task)

	var allChanged []string
	for _, repo := range targetRepos {
		localRepoPath := targetPath
		prefix := ""
		if errWS == nil {
			for i := range ws.Repos {
				if ws.Repos[i].RepoID == repo.ID {
					if worktreeSuffix != "" {
						role := r.GetRoleFromSuffix(worktreeSuffix)
						if relPath, exists := ws.Repos[i].Paths.Worktrees[role]; exists && relPath != "" {
							localRepoPath = filepath.Join(ws.Root, relPath)
						} else {
							localRepoPath = filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
						}
					} else {
						localRepoPath = filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
					}
					if task.RepositoryID == nil {
						prefix = ws.Repos[i].Name + "/"
					}
					break
				}
			}
		} else if task.RepositoryID == nil {
			parts := strings.Split(repo.URL, "/")
			repoName := parts[len(parts)-1]
			repoName = strings.TrimSuffix(repoName, ".git")
			localRepoPath = filepath.Join(targetPath, repoName)
			prefix = repoName + "/"
		}

		containerRepoPath := r.ContainerPathForHostPath(task, localRepoPath, "")
		repoChanged, err := r.SandboxGetChangedFiles(ctx, task, agent, containerRepoPath)
		if err == nil && len(repoChanged) > 0 {
			for _, line := range repoChanged {
				if line != "" {
					allChanged = append(allChanged, prefix+line)
				}
			}
		}
	}
	return allChanged, nil
}
