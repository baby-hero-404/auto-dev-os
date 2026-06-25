package repoutil

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TargetRepositoriesForTask(task *models.Task, repos []models.Repository) []models.Repository {
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

func (m *Manager) LoadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error) {
	if m.ListRepositories == nil {
		return nil, fmt.Errorf("list repositories function is not configured")
	}
	repos, err := m.ListRepositories(ctx, task.ProjectID)
	if err != nil {
		return nil, err
	}
	targetRepos := TargetRepositoriesForTask(task, repos)
	if task.RepositoryID != nil && len(targetRepos) == 0 {
		return nil, fmt.Errorf("task repository %s not found", *task.RepositoryID)
	}
	return targetRepos, nil
}
