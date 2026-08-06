package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// TaskAttemptRepo persists TaskAttempt rows — one per execution attempt of
// a Task (parent or child). Kept as its own repo (not folded into TaskRepo)
// since it has an independent lifecycle: attempts are created/finalized by
// the orchestrator around a single execution, not by the task CRUD surface.
type TaskAttemptRepo struct{ db *gorm.DB }

func NewTaskAttemptRepo(db *gorm.DB) *TaskAttemptRepo {
	return &TaskAttemptRepo{db: db}
}

// Create starts a new attempt for taskID. AttemptNumber is 1-based and
// computed as (count of existing attempts for this task) + 1, so retries
// always get the next sequential number without relying on caller bookkeeping.
func (r *TaskAttemptRepo) Create(ctx context.Context, taskID string, sandboxRef string) (*models.TaskAttempt, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.TaskAttempt{}).Where("task_id = ?", taskID).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count task attempts: %w", err)
	}
	a := &models.TaskAttempt{
		TaskID:        taskID,
		AttemptNumber: int(count) + 1,
		StartedAt:     time.Now().UTC(),
		SandboxRef:    sandboxRef,
	}
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("create task attempt: %w", err)
	}
	return a, nil
}

// Finalize records the terminal outcome (exit status, tokens, cost) of an
// in-flight attempt. Never mutates a prior attempt's identity — only the one
// row created for this specific execution.
func (r *TaskAttemptRepo) Finalize(ctx context.Context, id string, input models.FinalizeTaskAttemptInput) (*models.TaskAttempt, error) {
	now := time.Now().UTC()
	updates := map[string]any{"finished_at": now}
	if input.ExitStatus != nil {
		updates["exit_status"] = *input.ExitStatus
	}
	if input.TokensIn != nil {
		updates["tokens_in"] = *input.TokensIn
	}
	if input.TokensOut != nil {
		updates["tokens_out"] = *input.TokensOut
	}
	if input.CostUSD != nil {
		updates["cost_usd"] = *input.CostUSD
	}
	if err := r.db.WithContext(ctx).Model(&models.TaskAttempt{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("finalize task attempt: %w", err)
	}
	a := &models.TaskAttempt{}
	if err := r.db.WithContext(ctx).First(a, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get task attempt: %w", mapError(err))
	}
	return a, nil
}

func (r *TaskAttemptRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.TaskAttempt, error) {
	var attempts []models.TaskAttempt
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("attempt_number ASC").Find(&attempts).Error; err != nil {
		return nil, fmt.Errorf("list task attempts: %w", err)
	}
	return attempts, nil
}

func (r *TaskAttemptRepo) GetLatestByTaskID(ctx context.Context, taskID string) (*models.TaskAttempt, error) {
	a := &models.TaskAttempt{}
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("attempt_number DESC").First(a).Error; err != nil {
		return nil, fmt.Errorf("get latest task attempt: %w", mapError(err))
	}
	return a, nil
}

// ListByTaskIDs batches ListByTaskID across multiple tasks (used by Reduce
// to aggregate every child's attempts in one query instead of N).
func (r *TaskAttemptRepo) ListByTaskIDs(ctx context.Context, taskIDs []string) ([]models.TaskAttempt, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	var attempts []models.TaskAttempt
	if err := r.db.WithContext(ctx).Where("task_id IN ?", taskIDs).Order("task_id ASC, attempt_number ASC").Find(&attempts).Error; err != nil {
		return nil, fmt.Errorf("list task attempts by task ids: %w", err)
	}
	return attempts, nil
}
