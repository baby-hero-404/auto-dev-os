package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AgentService struct{ repo *repository.AgentRepo }

func NewAgentService(repo *repository.AgentRepo) *AgentService {
	return &AgentService{repo: repo}
}

func (s *AgentService) Create(ctx context.Context, projectID string, input models.CreateAgentInput) (*models.Agent, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *AgentService) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AgentService) ListByProjectID(ctx context.Context, projectID string) ([]models.Agent, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *AgentService) Update(ctx context.Context, id string, input models.UpdateAgentInput) (*models.Agent, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *AgentService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
