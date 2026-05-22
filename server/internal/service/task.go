package service

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type TaskService struct{ repo *repository.TaskRepo }

func NewTaskService(repo *repository.TaskRepo) *TaskService {
	return &TaskService{repo: repo}
}

func (s *TaskService) Create(ctx context.Context, projectID string, input models.CreateTaskInput) (*models.Task, error) {
	if input.Title == "" {
		return nil, ErrValidation("title is required")
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *TaskService) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) ListByProjectID(ctx context.Context, projectID string) ([]models.Task, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *TaskService) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	// Enforce task lifecycle state machine.
	if input.Status != nil {
		task, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := validateTransition(task.Status, *input.Status); err != nil {
			return nil, err
		}
	}
	return s.repo.Update(ctx, id, input)
}

func (s *TaskService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func validateTransition(from, to string) error {
	allowed, ok := models.ValidTaskTransitions[from]
	if !ok {
		return fmt.Errorf("validation: unknown current status %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("validation: invalid transition from %q to %q", from, to)
}
