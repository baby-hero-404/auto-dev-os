package wkspace

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
}

type RepositoryRepository interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error)
}

type GitOpsClient interface {
	CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error)
}

type ArtifactRepository interface {
	ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error)
	DeleteByTaskID(ctx context.Context, taskID string) error
}

type WorkflowRepository interface {
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
	AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error)
	ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error
	DeleteByTaskID(ctx context.Context, taskID string) error
}
