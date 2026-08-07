package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type TaskActionRequestRepo struct {
	db *gorm.DB
}

func NewTaskActionRequestRepo(db *gorm.DB) *TaskActionRequestRepo {
	return &TaskActionRequestRepo{db: db}
}

// FindByRequestID returns the stored request, or ErrNotFound if this
// (task_id, request_id) pair hasn't been seen yet.
func (r *TaskActionRequestRepo) FindByRequestID(ctx context.Context, taskID, requestID string) (*models.TaskActionRequest, error) {
	var req models.TaskActionRequest
	err := r.db.WithContext(ctx).Where("task_id = ? AND request_id = ?", taskID, requestID).First(&req).Error
	if err != nil {
		return nil, fmt.Errorf("find task action request: %w", mapError(err))
	}
	return &req, nil
}

// Create persists the request/response pair. A unique-constraint violation
// (concurrent double-submit racing this same call) maps to ErrConflict so
// the caller can fall back to re-reading the winning row.
func (r *TaskActionRequestRepo) Create(ctx context.Context, req *models.TaskActionRequest) error {
	if err := r.db.WithContext(ctx).Create(req).Error; err != nil {
		return fmt.Errorf("create task action request: %w", mapError(err))
	}
	return nil
}
