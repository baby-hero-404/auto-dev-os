package service

import (
	"context"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AgentService struct {
	repo          *repository.AgentRepo
	roleTemplates *repository.RoleTemplateRepo
}

func NewAgentService(repo *repository.AgentRepo) *AgentService {
	return &AgentService{repo: repo}
}

func (s *AgentService) WithRoleTemplateRepo(repo *repository.RoleTemplateRepo) *AgentService {
	s.roleTemplates = repo
	return s
}

func (s *AgentService) Create(ctx context.Context, projectID string, input models.CreateAgentInput) (*models.Agent, error) {
	prepared, err := s.prepareCreateInput(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, projectID, prepared)
}

func (s *AgentService) Hire(ctx context.Context, orgID string, input models.CreateAgentInput) (*models.Agent, error) {
	prepared, err := s.prepareCreateInput(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateForOrg(ctx, orgID, prepared)
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

func (s *AgentService) ListRoleTemplates(ctx context.Context) ([]models.RoleTemplate, error) {
	if s.roleTemplates == nil {
		return []models.RoleTemplate{}, nil
	}
	return s.roleTemplates.ListAll(ctx)
}

func (s *AgentService) Update(ctx context.Context, id string, input models.UpdateAgentInput) (*models.Agent, error) {
	if input.AutonomyLevel != nil {
		if err := validateAgentAutonomyLevel(*input.AutonomyLevel); err != nil {
			return nil, err
		}
	}
	if input.AssignmentStrategy != nil {
		if err := validateAgentAssignmentStrategy(*input.AssignmentStrategy); err != nil {
			return nil, err
		}
	}
	if input.Role != nil && strings.TrimSpace(*input.Role) == "" {
		return nil, ErrValidation("role is required")
	}
	if input.Goal != nil && strings.TrimSpace(*input.Goal) == "" {
		return nil, ErrValidation("goal is required")
	}
	if input.ModelRoute != nil && strings.TrimSpace(*input.ModelRoute) == "" {
		defaultRoute := "balanced"
		input.ModelRoute = &defaultRoute
	}
	return s.repo.Update(ctx, id, input)
}

func (s *AgentService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *AgentService) prepareCreateInput(ctx context.Context, input models.CreateAgentInput) (models.CreateAgentInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.TrimSpace(input.Role)
	input.Goal = strings.TrimSpace(input.Goal)
	input.ModelRoute = strings.TrimSpace(input.ModelRoute)
	input.AutonomyLevel = strings.TrimSpace(input.AutonomyLevel)

	if input.Name == "" {
		return input, ErrValidation("name is required")
	}
	if input.Role == "" {
		return input, ErrValidation("role is required")
	}
	if input.ModelRoute == "" {
		input.ModelRoute = "balanced"
	}
	if input.AutonomyLevel == "" {
		input.AutonomyLevel = models.AgentAutonomySupervised
	}
	if err := validateAgentAutonomyLevel(input.AutonomyLevel); err != nil {
		return input, err
	}
	if err := validateAgentAssignmentStrategy(input.AssignmentStrategy); err != nil {
		return input, err
	}
	if len(input.ContextConfig) == 0 {
		input.ContextConfig = []byte(`{"max_input_tokens":128000}`)
	}

	if s.roleTemplates != nil {
		if template, err := s.roleTemplates.GetByRole(ctx, input.Role); err == nil && template != nil {
			if input.Goal == "" {
				input.Goal = template.DefaultGoal
			}
		}
	}
	if input.Goal == "" {
		return input, ErrValidation("goal is required")
	}
	return input, nil
}

func validateAgentAssignmentStrategy(strategy string) error {
	switch strategy {
	case "", models.AgentAssignmentManual, models.AgentAssignmentAutoJoin:
		return nil
	default:
		return ErrValidation("assignment_strategy must be manual or auto_join")
	}
}

func validateAgentAutonomyLevel(level string) error {
	switch level {
	case models.AgentAutonomyAutonomous, models.AgentAutonomySupervised, models.AgentAutonomyApprovalRequired:
		return nil
	default:
		return ErrValidation("autonomy_level must be autonomous, supervised, or approval_required")
	}
}
