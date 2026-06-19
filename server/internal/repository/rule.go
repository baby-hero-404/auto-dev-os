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

func (r *RuleRepo) CreateGlobal(ctx context.Context, orgID string, input models.CreateRuleInput) (*models.Rule, error) {
	scope := input.Scope
	if scope == "" {
		scope = models.RuleScopeGlobal
	}
	rule := &models.Rule{
		OrgID:       &orgID,
		Scope:       scope,
		Content:     input.Content,
		Enforcement: input.Enforcement,
	}
	if err := r.db.WithContext(ctx).Create(rule).Error; err != nil {
		return nil, fmt.Errorf("create global rule: %w", err)
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

func (r *RuleRepo) GetByIDAndOrg(ctx context.Context, id string, orgID string) (*models.Rule, error) {
	rule := &models.Rule{}
	err := r.db.WithContext(ctx).
		Where("id = ? AND (org_id = ? OR project_id IN (SELECT id FROM projects WHERE org_id = ?))", id, orgID, orgID).
		First(rule).Error
	if err != nil {
		return nil, fmt.Errorf("get rule by id and org: %w", mapError(err))
	}
	return rule, nil
}

func (r *RuleRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Rule, error) {
	var rules []models.Rule
	if err := r.db.WithContext(ctx).
		Where(`project_id = ? OR (scope = ? AND org_id = (SELECT org_id FROM projects WHERE id = ?))`, projectID, models.RuleScopeGlobal, projectID).
		Order("scope, created_at DESC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	return rules, nil
}

func (r *RuleRepo) ListGlobalByOrgID(ctx context.Context, orgID string) ([]models.Rule, error) {
	var rules []models.Rule
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND scope = ?", orgID, models.RuleScopeGlobal).
		Order("created_at DESC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list global rules: %w", err)
	}
	return rules, nil
}

func (r *RuleRepo) Update(ctx context.Context, id string, orgID string, input models.UpdateRuleInput) (*models.Rule, error) {
	rule, err := r.GetByIDAndOrg(ctx, id, orgID)
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

func (r *RuleRepo) Delete(ctx context.Context, id string, orgID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND (org_id = ? OR project_id IN (SELECT id FROM projects WHERE org_id = ?))", id, orgID, orgID).
		Delete(&models.Rule{})
	if result.Error != nil {
		return fmt.Errorf("delete rule: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete rule: %w", ErrNotFound)
	}
	return nil
}
