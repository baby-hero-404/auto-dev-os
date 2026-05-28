package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type Queue struct {
	repo *repository.WorkflowRepo
}

func NewQueue(repo *repository.WorkflowRepo) *Queue {
	return &Queue{repo: repo}
}

func (q *Queue) Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	return q.repo.Enqueue(ctx, taskID)
}

func (q *Queue) ClaimNext(ctx context.Context) (*models.WorkflowJob, error) {
	return q.repo.ClaimNext(ctx)
}
