package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type ProviderModelRepo struct {
	db *gorm.DB
}

func NewProviderModelRepo(db *gorm.DB) *ProviderModelRepo {
	return &ProviderModelRepo{db: db}
}

func (r *ProviderModelRepo) Create(ctx context.Context, orgID string, input models.CreateProviderModelInput) (*models.ProviderModel, error) {
	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	model := &models.ProviderModel{
		OrgID:      orgID,
		Provider:   input.Provider,
		LevelGroup: input.LevelGroup,
		ModelName:  input.ModelName,
		Priority:   input.Priority,
		IsActive:   isActive,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return nil, fmt.Errorf("create provider model: %w", mapError(err))
	}
	return model, nil
}

func (r *ProviderModelRepo) CreateBatch(ctx context.Context, modelsList []models.ProviderModel) error {
	if len(modelsList) == 0 {
		return nil
	}
	// Use ON CONFLICT DO NOTHING to avoid duplicate key violations during race conditions
	if err := r.db.WithContext(ctx).Clauses(gorm.Expr("ON CONFLICT (org_id, provider, level_group, model_name) DO NOTHING")).Create(&modelsList).Error; err != nil {
		return fmt.Errorf("create batch provider models: %w", err)
	}
	return nil
}

func (r *ProviderModelRepo) ListByOrg(ctx context.Context, orgID string, filter models.ProviderModelFilter) ([]models.ProviderModel, error) {
	var list []models.ProviderModel
	query := r.db.WithContext(ctx).Where("org_id = ?", orgID)
	if filter.Provider != nil && *filter.Provider != "" {
		query = query.Where("provider = ?", *filter.Provider)
	}
	if filter.LevelGroup != nil && *filter.LevelGroup != "" {
		query = query.Where("level_group = ?", *filter.LevelGroup)
	}
	if err := query.Order("priority ASC, created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list provider models: %w", err)
	}
	return list, nil
}

func (r *ProviderModelRepo) Get(ctx context.Context, orgID string, id string) (*models.ProviderModel, error) {
	model := &models.ProviderModel{}
	if err := r.db.WithContext(ctx).First(model, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return nil, fmt.Errorf("get provider model: %w", mapError(err))
	}
	return model, nil
}

func (r *ProviderModelRepo) Update(ctx context.Context, orgID string, id string, input models.UpdateProviderModelInput) (*models.ProviderModel, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Provider != nil {
		updates["provider"] = *input.Provider
	}
	if input.LevelGroup != nil {
		updates["level_group"] = *input.LevelGroup
	}
	if input.ModelName != nil {
		updates["model_name"] = *input.ModelName
	}
	if input.Priority != nil {
		updates["priority"] = *input.Priority
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	result := r.db.WithContext(ctx).Model(&models.ProviderModel{}).Where("id = ? AND org_id = ?", id, orgID).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update provider model: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("update provider model: %w", ErrNotFound)
	}

	model := &models.ProviderModel{}
	if err := r.db.WithContext(ctx).First(model, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return nil, fmt.Errorf("get updated provider model: %w", mapError(err))
	}
	return model, nil
}

func (r *ProviderModelRepo) Delete(ctx context.Context, orgID string, id string) error {
	result := r.db.WithContext(ctx).Where("org_id = ?", orgID).Delete(&models.ProviderModel{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete provider model: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete provider model: %w", ErrNotFound)
	}
	return nil
}
