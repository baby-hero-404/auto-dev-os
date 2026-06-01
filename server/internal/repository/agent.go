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
	a := &models.Agent{
		OrgID: orgID, Name: input.Name, Role: input.Role,
		Provider: input.Provider, Model: input.Model, Level: input.Level,
		AssignmentStrategy: strategy,
	}
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
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

func (r *AgentRepo) Update(ctx context.Context, id string, input models.UpdateAgentInput) (*models.Agent, error) {
	a, err := r.GetByID(ctx, id)
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
	if input.Provider != nil {
		updates["provider"] = *input.Provider
	}
	if input.Model != nil {
		updates["model"] = *input.Model
	}
	if input.Level != nil {
		updates["level"] = *input.Level
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.AssignmentStrategy != nil {
		updates["assignment_strategy"] = *input.AssignmentStrategy
	}
	if err := r.db.WithContext(ctx).Model(a).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	return a, nil
}

func (r *AgentRepo) FindAvailableForTask(ctx context.Context, projectID, complexity string) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).
		Table("agents").
		Joins("JOIN projects ON projects.org_id = agents.org_id").
		Where("projects.id = ?", projectID).
		Where("agents.status IN ?", []string{models.AgentStatusIdle, ""}).
		Where("agents.assignment_strategy = ? OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = ?)", models.AgentAssignmentAutoJoin, projectID).
		Order(agentLevelOrderSQL(complexity)).
		Order("agents.created_at ASC").
		First(&agent).Error
	if err != nil {
		return nil, fmt.Errorf("find available agent: %w", mapError(err))
	}
	return &agent, nil
}

func (r *AgentRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Agent{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete agent: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete agent: %w", ErrNotFound)
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

func agentLevelOrderSQL(complexity string) string {
	switch complexity {
	case models.TaskComplexityHard:
		return "CASE agents.level WHEN 'hard' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END"
	case models.TaskComplexityMedium:
		return "CASE agents.level WHEN 'medium' THEN 0 WHEN 'hard' THEN 1 ELSE 2 END"
	default:
		return "CASE agents.level WHEN 'easy' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END"
	}
}
