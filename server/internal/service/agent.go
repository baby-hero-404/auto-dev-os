package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AgentService struct {
	repo          *repository.AgentRepo
	roleTemplates *repository.RoleTemplateRepo
	skills        *repository.SkillRepo
}

func NewAgentService(repo *repository.AgentRepo) *AgentService {
	return &AgentService{repo: repo}
}

func (s *AgentService) WithRoleTemplateRepo(repo *repository.RoleTemplateRepo) *AgentService {
	s.roleTemplates = repo
	return s
}

func (s *AgentService) WithSkillRepo(repo *repository.SkillRepo) *AgentService {
	s.skills = repo
	return s
}

func (s *AgentService) Create(ctx context.Context, projectID string, input models.CreateAgentInput) (*models.Agent, error) {
	prepared, err := s.prepareCreateInput(ctx, input)
	if err != nil {
		return nil, err
	}
	agent, err := s.repo.Create(ctx, projectID, prepared)
	if err != nil {
		return nil, err
	}
	if err := s.assignSkills(ctx, agent.ID, prepared.SkillIDs); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentService) Hire(ctx context.Context, orgID string, input models.CreateAgentInput) (*models.Agent, error) {
	prepared, err := s.prepareCreateInput(ctx, input)
	if err != nil {
		return nil, err
	}
	agent, err := s.repo.CreateForOrg(ctx, orgID, prepared)
	if err != nil {
		return nil, err
	}
	if err := s.assignSkills(ctx, agent.ID, prepared.SkillIDs); err != nil {
		return nil, err
	}
	return agent, nil
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
	agent, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	if input.SkillIDs != nil {
		if err := s.assignSkills(ctx, agent.ID, input.SkillIDs); err != nil {
			return nil, err
		}
	}
	return agent, nil
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
			if len(input.SkillIDs) == 0 && s.skills != nil {
				var toolNames []string
				if err := json.Unmarshal(template.DefaultTools, &toolNames); err == nil {
					skills, err := s.skills.ListByNames(ctx, toolNames)
					if err != nil {
						return input, err
					}
					for _, skill := range skills {
						input.SkillIDs = append(input.SkillIDs, skill.ID)
					}
				}
			}
		}
	}
	if input.Goal == "" {
		return input, ErrValidation("goal is required")
	}
	return input, nil
}

func (s *AgentService) assignSkills(ctx context.Context, agentID string, skillIDs []string) error {
	if s.skills == nil || skillIDs == nil {
		return nil
	}
	return s.skills.ReplaceAgentSkills(ctx, agentID, skillIDs)
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
