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
	if err := validateAgentAssignmentStrategy(input.AssignmentStrategy); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *AgentService) Hire(ctx context.Context, orgID string, input models.CreateAgentInput) (*models.Agent, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	if err := validateAgentAssignmentStrategy(input.AssignmentStrategy); err != nil {
		return nil, err
	}
	return s.repo.CreateForOrg(ctx, orgID, input)
}

func (s *AgentService) AssignToProject(ctx context.Context, projectID string, input models.CreateAgentInput) (*models.Agent, error) {
	if input.AgentID != "" {
		if err := s.repo.AssignToProject(ctx, projectID, input.AgentID); err != nil {
			return nil, err
		}
		return s.repo.GetByID(ctx, input.AgentID)
	}
	return s.Create(ctx, projectID, input)
}

func (s *AgentService) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AgentService) ListByProjectID(ctx context.Context, projectID string) ([]models.Agent, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *AgentService) ListByOrgID(ctx context.Context, orgID string) ([]models.Agent, error) {
	return s.repo.ListByOrgID(ctx, orgID)
}

func (s *AgentService) Update(ctx context.Context, id string, input models.UpdateAgentInput) (*models.Agent, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *AgentService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func validateAgentAssignmentStrategy(strategy string) error {
	switch strategy {
	case "", models.AgentAssignmentManual, models.AgentAssignmentAutoJoin:
		return nil
	default:
		return ErrValidation("assignment_strategy must be manual or auto_join")
	}
}
