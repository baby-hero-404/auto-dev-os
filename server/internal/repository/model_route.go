package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type ModelRouteRepo struct{ db *gorm.DB }

func NewModelRouteRepo(db *gorm.DB) *ModelRouteRepo {
	return &ModelRouteRepo{db: db}
}

func (r *ModelRouteRepo) Create(ctx context.Context, orgID string, input models.CreateModelRouteInput) (*models.ModelRoute, error) {
	route := &models.ModelRoute{
		OrgID:     orgID,
		Name:      input.Name,
		RouteType: input.RouteType,
		Config:    input.Config,
		IsDefault: input.IsDefault,
	}
	if err := r.db.WithContext(ctx).Create(route).Error; err != nil {
		return nil, fmt.Errorf("create model route: %w", err)
	}
	return route, nil
}

func (r *ModelRouteRepo) ListByOrg(ctx context.Context, orgID string) ([]models.ModelRoute, error) {
	var routes []models.ModelRoute
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("is_default DESC, created_at DESC").Find(&routes).Error; err != nil {
		return nil, fmt.Errorf("list model routes: %w", err)
	}
	return routes, nil
}

func (r *ModelRouteRepo) GetDefault(ctx context.Context, orgID string) (*models.ModelRoute, error) {
	route := &models.ModelRoute{}
	if err := r.db.WithContext(ctx).First(route, "org_id = ? AND is_default = TRUE", orgID).Error; err != nil {
		return nil, fmt.Errorf("get default model route: %w", mapError(err))
	}
	return route, nil
}

func (r *ModelRouteRepo) GetByName(ctx context.Context, orgID, name string) (*models.ModelRoute, error) {
	route := &models.ModelRoute{}
	if err := r.db.WithContext(ctx).First(route, "org_id = ? AND name = ?", orgID, name).Error; err != nil {
		return nil, fmt.Errorf("get model route: %w", mapError(err))
	}
	return route, nil
}

func (r *ModelRouteRepo) Update(ctx context.Context, id string, input models.UpdateModelRouteInput) (*models.ModelRoute, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.RouteType != nil {
		updates["route_type"] = *input.RouteType
	}
	if input.Config != nil {
		updates["config"] = *input.Config
	}
	if input.IsDefault != nil {
		updates["is_default"] = *input.IsDefault
	}
	result := r.db.WithContext(ctx).Model(&models.ModelRoute{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update model route: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("update model route: %w", ErrNotFound)
	}
	route := &models.ModelRoute{}
	if err := r.db.WithContext(ctx).First(route, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get updated model route: %w", mapError(err))
	}
	return route, nil
}

func (r *ModelRouteRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.ModelRoute{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete model route: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete model route: %w", ErrNotFound)
	}
	return nil
}
