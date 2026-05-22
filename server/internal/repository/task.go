package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type TaskRepo struct{ db *gorm.DB }

func NewTaskRepo(db *gorm.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(ctx context.Context, projectID string, input models.CreateTaskInput) (*models.Task, error) {
	t := &models.Task{
		ProjectID: projectID, Title: input.Title, Description: input.Description,
		Complexity: input.Complexity, Priority: input.Priority,
		Labels: pq.StringArray(input.Labels),
	}
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (r *TaskRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	t := &models.Task{}
	if err := r.db.WithContext(ctx).First(t, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *TaskRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Task, error) {
	var tasks []models.Task
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("priority DESC, created_at DESC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepo) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	t, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Complexity != nil {
		updates["complexity"] = *input.Complexity
	}
	if input.Priority != nil {
		updates["priority"] = *input.Priority
	}
	if input.AgentID != nil {
		updates["agent_id"] = *input.AgentID
	}
	if input.Labels != nil {
		updates["labels"] = pq.StringArray(input.Labels)
	}
	if err := r.db.WithContext(ctx).Model(t).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	return t, nil
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Task{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}
