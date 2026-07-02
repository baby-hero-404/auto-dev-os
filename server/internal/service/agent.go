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

func (s *AgentService) GetByID(ctx context.Context, id string, orgID string) (*models.Agent, error) {
	return s.repo.GetByIDAndOrg(ctx, id, orgID)
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

func (s *AgentService) Update(ctx context.Context, id string, orgID string, input models.UpdateAgentInput) (*models.Agent, error) {
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
	if input.Role != nil {
		if err := validateAgentRole(*input.Role); err != nil {
			return nil, err
		}
	}
	if input.Goal != nil && strings.TrimSpace(*input.Goal) == "" {
		return nil, ErrValidation("goal is required")
	}
	if input.ModelLevelGroup != nil {
		trimmed := strings.TrimSpace(*input.ModelLevelGroup)
		if trimmed == "" {
			role := ""
			if input.Role != nil && strings.TrimSpace(*input.Role) != "" {
				role = strings.TrimSpace(*input.Role)
			} else {
				currentAgent, err := s.repo.GetByIDAndOrg(ctx, id, orgID)
				if err != nil {
					return nil, err
				}
				role = currentAgent.Role
			}
			defaultLevelGroup := getDefaultModelLevelGroupForRole(role)
			input.ModelLevelGroup = &defaultLevelGroup
		} else {
			*input.ModelLevelGroup = trimmed
		}
		if err := validateAgentModelLevelGroup(*input.ModelLevelGroup); err != nil {
			return nil, err
		}
	}
	return s.repo.Update(ctx, id, orgID, input)
}

func (s *AgentService) Delete(ctx context.Context, id string, orgID string) error {
	return s.repo.Delete(ctx, id, orgID)
}

func (s *AgentService) prepareCreateInput(ctx context.Context, input models.CreateAgentInput) (models.CreateAgentInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.TrimSpace(input.Role)
	input.Goal = strings.TrimSpace(input.Goal)
	input.ModelLevelGroup = strings.TrimSpace(input.ModelLevelGroup)
	input.AutonomyLevel = strings.TrimSpace(input.AutonomyLevel)

	if input.Name == "" {
		return input, ErrValidation("name is required")
	}
	if input.Role == "" {
		return input, ErrValidation("role is required")
	}
	if err := validateAgentRole(input.Role); err != nil {
		return input, err
	}
	if input.ModelLevelGroup == "" {
		input.ModelLevelGroup = getDefaultModelLevelGroupForRole(input.Role)
	}
	if err := validateAgentModelLevelGroup(input.ModelLevelGroup); err != nil {
		return input, err
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
		if input.Role == models.AgentRoleDocumentationWriter {
			input.Goal = "Write and update high-quality project documentation, readme files, and user guides."
		} else {
			return input, ErrValidation("goal is required")
		}
	}
	return input, nil
}

func validateAgentRole(role string) error {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case models.AgentRolePlanner,
		models.AgentRoleBackend,
		models.AgentRoleFrontend,
		models.AgentRoleReviewer,
		models.AgentRoleQA,
		models.AgentRoleSecurityAuditor,
		models.AgentRoleDBArchitect,
		models.AgentRoleDocumentationWriter:
		return nil
	default:
		return ErrValidation("role must be planner, backend, frontend, reviewer, qa, security-auditor, db-architect, or documentation-writer")
	}
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

func validateAgentModelLevelGroup(levelGroup string) error {
	switch levelGroup {
	case models.ModelLevelFast, models.ModelLevelBalanced, models.ModelLevelPowerful, "auto":
		return nil
	default:
		return ErrValidation("model_level_group must be fast, balanced, powerful, or auto")
	}
}

func getDefaultModelLevelGroupForRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case models.AgentRolePlanner, models.AgentRoleDBArchitect:
		return models.ModelLevelPowerful
	case models.AgentRoleBackend, models.AgentRoleFrontend, models.AgentRoleDocumentationWriter:
		return models.ModelLevelBalanced
	case models.AgentRoleReviewer:
		return models.ModelLevelFast
	case models.AgentRoleQA:
		return models.ModelLevelBalanced
	case models.AgentRoleSecurityAuditor:
		return models.ModelLevelPowerful
	default:
		return models.ModelLevelBalanced
	}
}
