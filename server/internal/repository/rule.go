package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type RuleRepo struct{ db *gorm.DB }

func NewRuleRepo(db *gorm.DB) *RuleRepo {
	return &RuleRepo{db: db}
}

func (r *RuleRepo) Create(ctx context.Context, projectID *string, input models.CreateRuleInput) (*models.Rule, error) {
	rule := &models.Rule{
		ProjectID: projectID, Scope: input.Scope,
		Content: input.Content, Enforcement: input.Enforcement,
	}
	if err := r.db.WithContext(ctx).Create(rule).Error; err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}
	return rule, nil
}

func (r *RuleRepo) GetByID(ctx context.Context, id string) (*models.Rule, error) {
	rule := &models.Rule{}
	if err := r.db.WithContext(ctx).First(rule, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get rule: %w", mapError(err))
	}
	return rule, nil
}

func (r *RuleRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Rule, error) {
	var rules []models.Rule
	if err := r.db.WithContext(ctx).Where("project_id = ? OR scope = 'global'", projectID).Order("scope, created_at DESC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	return rules, nil
}

func (r *RuleRepo) Update(ctx context.Context, id string, input models.UpdateRuleInput) (*models.Rule, error) {
	rule, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.Content != nil {
		updates["content"] = *input.Content
	}
	if input.Enforcement != nil {
		updates["enforcement"] = *input.Enforcement
	}
	if err := r.db.WithContext(ctx).Model(rule).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}
	return rule, nil
}

func (r *RuleRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Rule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete rule: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete rule: %w", ErrNotFound)
	}
	return nil
}
