package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowRepo struct{ db *gorm.DB }

func NewWorkflowRepo(db *gorm.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db}
}

func (r *WorkflowRepo) Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	job := &models.WorkflowJob{TaskID: taskID, Status: models.WorkflowJobStatusQueued, Step: models.WorkflowStepAnalyze}
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("enqueue workflow job: %w", err)
	}
	return job, nil
}

func (r *WorkflowRepo) ClaimNext(ctx context.Context) (*models.WorkflowJob, error) {
	var job models.WorkflowJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", models.WorkflowJobStatusQueued).
			Order("created_at ASC").
			First(&job).Error; err != nil {
			return err
		}

		return tx.Model(&job).Updates(map[string]any{
			"status":   models.WorkflowJobStatusRunning,
			"attempts": gorm.Expr("attempts + 1"),
		}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim workflow job: %w", err)
	}
	return &job, nil
}

func (r *WorkflowRepo) LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	var job models.WorkflowJob
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").First(&job).Error; err != nil {
		return nil, fmt.Errorf("latest workflow job: %w", err)
	}
	return &job, nil
}

func (r *WorkflowRepo) UpdateJob(ctx context.Context, jobID string, updates map[string]any) (*models.WorkflowJob, error) {
	var job models.WorkflowJob
	if err := r.db.WithContext(ctx).Model(&models.WorkflowJob{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update workflow job: %w", err)
	}
	if err := r.db.WithContext(ctx).First(&job, "id = ?", jobID).Error; err != nil {
		return nil, fmt.Errorf("reload workflow job: %w", err)
	}
	return &job, nil
}

func (r *WorkflowRepo) CreateCheckpoint(ctx context.Context, checkpoint models.WorkflowCheckpoint) error {
	if err := r.db.WithContext(ctx).Create(&checkpoint).Error; err != nil {
		return fmt.Errorf("create workflow checkpoint: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	var checkpoints []models.WorkflowCheckpoint
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&checkpoints).Error; err != nil {
		return nil, fmt.Errorf("list workflow checkpoints: %w", err)
	}
	return checkpoints, nil
}

func (r *WorkflowRepo) CreateLog(ctx context.Context, log models.TaskLog) error {
	if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
		return fmt.Errorf("create task log: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	var logs []models.TaskLog
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list task logs: %w", err)
	}
	return logs, nil
}
