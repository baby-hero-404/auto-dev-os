package checkpoint

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type CheckpointRepo interface {
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
}

type ArtifactRepo interface {
	ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error)
	Create(ctx context.Context, artifact *models.WorkflowArtifact) error
}
