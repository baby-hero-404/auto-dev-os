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
	// TokenForRepoURL resolves the same read credential CloneForTask uses,
	// without cloning. Used to provision the read-only git credential
	// helper agents call mid-task (see writeGitCredentialHelper).
	TokenForRepoURL(ctx context.Context, repoURL string) (string, error)
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
