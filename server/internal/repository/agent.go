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
	a := &models.Agent{
		ProjectID: projectID, Name: input.Name, Role: input.Role,
		Provider: input.Provider, Model: input.Model, Level: input.Level,
	}
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return a, nil
}

func (r *AgentRepo) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	a := &models.Agent{}
	if err := r.db.WithContext(ctx).First(a, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return a, nil
}

func (r *AgentRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Agent, error) {
	var agents []models.Agent
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return agents, nil
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
	if err := r.db.WithContext(ctx).Model(a).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	return a, nil
}

func (r *AgentRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Agent{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}
	return nil
}
