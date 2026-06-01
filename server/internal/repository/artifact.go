package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type ArtifactRepo struct {
	db *gorm.DB
}

func NewArtifactRepo(db *gorm.DB) *ArtifactRepo {
	return &ArtifactRepo{db: db}
}

func (r *ArtifactRepo) Create(ctx context.Context, artifact *models.WorkflowArtifact) error {
	if err := r.db.WithContext(ctx).Create(artifact).Error; err != nil {
		return fmt.Errorf("create workflow artifact: %w", mapError(err))
	}
	return nil
}

func (r *ArtifactRepo) ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error) {
	var artifacts []models.WorkflowArtifact
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Order("created_at ASC").Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("list workflow artifacts by job: %w", mapError(err))
	}
	return artifacts, nil
}

func (r *ArtifactRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error) {
	var artifacts []models.WorkflowArtifact
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("list workflow artifacts by task: %w", mapError(err))
	}
	return artifacts, nil
}
