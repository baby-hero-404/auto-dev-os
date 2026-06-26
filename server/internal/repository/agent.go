package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type AgentRepo struct{ db *gorm.DB }

func NewAgentRepo(db *gorm.DB) *AgentRepo {
	return &AgentRepo{db: db}
}

func (r *AgentRepo) Create(ctx context.Context, projectID string, input models.CreateAgentInput) (*models.Agent, error) {
	orgID, err := r.orgIDForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	input.AssignmentStrategy = models.AgentAssignmentManual
	a, err := r.CreateForOrg(ctx, orgID, input)
	if err != nil {
		return nil, err
	}
	if err := r.AssignToProject(ctx, projectID, a.ID); err != nil {
		return nil, err
	}
	return a, nil
}

func (r *AgentRepo) CreateForOrg(ctx context.Context, orgID string, input models.CreateAgentInput) (*models.Agent, error) {
	strategy := input.AssignmentStrategy
	if strategy == "" {
		strategy = models.AgentAssignmentManual
	}
	contextConfig := input.ContextConfig
	if len(contextConfig) == 0 {
		contextConfig = []byte(`{"max_input_tokens":128000}`)
	}
	a := &models.Agent{
		OrgID:              orgID,
		Name:               input.Name,
		Role:               input.Role,
		Goal:               input.Goal,
		AutonomyLevel:      input.AutonomyLevel,
		ContextConfig:      contextConfig,
		ModelLevelGroup:    input.ModelLevelGroup,
		Status:             models.AgentStatusIdle,
		AssignmentStrategy: strategy,
	}
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("create agent: %w", mapError(err))
	}
	return a, nil
}

func (r *AgentRepo) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	a := &models.Agent{}
	if err := r.db.WithContext(ctx).First(a, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get agent: %w", mapError(err))
	}
	return a, nil
}

func (r *AgentRepo) GetByIDAndOrg(ctx context.Context, id string, orgID string) (*models.Agent, error) {
	a := &models.Agent{}
	if err := r.db.WithContext(ctx).First(a, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return nil, fmt.Errorf("get agent by org: %w", mapError(err))
	}
	return a, nil
}

func (r *AgentRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Agent, error) {
	var agents []models.Agent
	err := r.db.WithContext(ctx).
		Table("agents").
		Joins("JOIN projects ON projects.org_id = agents.org_id").
		Where("projects.id = ?", projectID).
		Where("agents.assignment_strategy = ? OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = ?)", models.AgentAssignmentAutoJoin, projectID).
		Order("agents.created_at DESC").
		Find(&agents).Error
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return agents, nil
}

func (r *AgentRepo) ListByOrgID(ctx context.Context, orgID string) ([]models.Agent, error) {
	var agents []models.Agent
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("list organization agents: %w", err)
	}
	return agents, nil
}

func (r *AgentRepo) AssignToProject(ctx context.Context, projectID, agentID string) error {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("agents").
		Joins("JOIN projects ON projects.org_id = agents.org_id").
		Where("projects.id = ? AND agents.id = ?", projectID, agentID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check agent project assignment: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("agent does not belong to project organization")
	}
	assignment := models.ProjectAgent{ProjectID: projectID, AgentID: agentID}
	if err := r.db.WithContext(ctx).FirstOrCreate(&assignment, "project_id = ? AND agent_id = ?", projectID, agentID).Error; err != nil {
		return fmt.Errorf("assign agent to project: %w", err)
	}
	return nil
}

func (r *AgentRepo) Update(ctx context.Context, id string, orgID string, input models.UpdateAgentInput) (*models.Agent, error) {
	a, err := r.GetByIDAndOrg(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Role != nil {
		updates["role"] = *input.Role
	}
	if input.Goal != nil {
		updates["goal"] = *input.Goal
	}
	if input.AutonomyLevel != nil {
		updates["autonomy_level"] = *input.AutonomyLevel
	}
	if input.ContextConfig != nil {
		updates["context_config"] = *input.ContextConfig
	}
	if input.ModelLevelGroup != nil {
		updates["model_level_group"] = *input.ModelLevelGroup
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.AssignmentStrategy != nil {
		updates["assignment_strategy"] = *input.AssignmentStrategy
	}
	if len(updates) == 0 {
		return a, nil
	}
	if err := r.db.WithContext(ctx).Model(a).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update agent: %w", mapError(err))
	}
	return r.GetByIDAndOrg(ctx, id, orgID)
}

func (r *AgentRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	if err := r.db.WithContext(ctx).Model(&models.Agent{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return fmt.Errorf("update agent status: %w", mapError(err))
	}
	return nil
}

func (r *AgentRepo) FindAvailableByRole(ctx context.Context, projectID, role string) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).
		Table("agents").
		Joins("JOIN projects ON projects.org_id = agents.org_id").
		Where("projects.id = ?", projectID).
		Where("agents.role = ?", role).
		Where("agents.status IN ?", []string{models.AgentStatusIdle, ""}).
		Where("agents.assignment_strategy = ? OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = ?)", models.AgentAssignmentAutoJoin, projectID).
		Order("agents.created_at ASC").
		First(&agent).Error
	if err != nil {
		return nil, fmt.Errorf("find available agent by role: %w", mapError(err))
	}
	return &agent, nil
}

func (r *AgentRepo) FindByRole(ctx context.Context, projectID, role string) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).
		Table("agents").
		Joins("JOIN projects ON projects.org_id = agents.org_id").
		Where("projects.id = ?", projectID).
		Where("agents.role = ?", role).
		Where("agents.assignment_strategy = ? OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = ?)", models.AgentAssignmentAutoJoin, projectID).
		Order("agents.created_at ASC").
		First(&agent).Error
	if err != nil {
		return nil, fmt.Errorf("find agent by role: %w", mapError(err))
	}
	return &agent, nil
}

func (r *AgentRepo) FindAnyAvailable(ctx context.Context, projectID string) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).
		Table("agents").
		Joins("JOIN projects ON projects.org_id = agents.org_id").
		Where("projects.id = ?", projectID).
		Where("agents.status IN ?", []string{models.AgentStatusIdle, ""}).
		Where("agents.assignment_strategy = ? OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = ?)", models.AgentAssignmentAutoJoin, projectID).
		Order("agents.created_at ASC").
		First(&agent).Error
	if err != nil {
		return nil, fmt.Errorf("find any available agent: %w", mapError(err))
	}
	return &agent, nil
}

func (r *AgentRepo) Delete(ctx context.Context, id string, orgID string) error {
	result := r.db.WithContext(ctx).Delete(&models.Agent{}, "id = ? AND org_id = ?", id, orgID)
	if result.Error != nil {
		return fmt.Errorf("delete agent: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete agent: %w", ErrNotFound)
	}
	return nil
}

func (r *AgentRepo) ResetAllStatuses(ctx context.Context) error {
	err := r.db.WithContext(ctx).Model(&models.Agent{}).
		Where("status IN ?", []string{models.AgentStatusAssigned, models.AgentStatusRunning}).
		Update("status", models.AgentStatusIdle).Error
	if err != nil {
		return fmt.Errorf("reset all agent statuses: %w", mapError(err))
	}
	return nil
}

func (r *AgentRepo) orgIDForProject(ctx context.Context, projectID string) (string, error) {
	var orgID string
	if err := r.db.WithContext(ctx).Table("projects").Select("org_id").Where("id = ?", projectID).Scan(&orgID).Error; err != nil {
		return "", fmt.Errorf("get project org: %w", mapError(err))
	}
	if orgID == "" {
		return "", fmt.Errorf("get project org: %w", ErrNotFound)
	}
	return orgID, nil
}
