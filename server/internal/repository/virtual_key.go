package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type VirtualKeyRepo struct{ db *gorm.DB }

func NewVirtualKeyRepo(db *gorm.DB) *VirtualKeyRepo {
	return &VirtualKeyRepo{db: db}
}

func (r *VirtualKeyRepo) Create(ctx context.Context, orgID string, input models.CreateVirtualKeyInput, keyHash, keyPrefix string) (*models.VirtualKey, error) {
	key := &models.VirtualKey{
		OrgID:          orgID,
		ProjectID:      input.ProjectID,
		AgentID:        input.AgentID,
		KeyHash:        keyHash,
		KeyPrefix:      keyPrefix,
		Name:           input.Name,
		BudgetLimitUSD: input.BudgetLimitUSD,
		RPMLimit:       input.RPMLimit,
		TPMLimit:       input.TPMLimit,
		Status:         models.VirtualKeyStatusActive,
		ExpiresAt:      input.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(key).Error; err != nil {
		return nil, fmt.Errorf("create virtual key: %w", err)
	}
	return key, nil
}

func (r *VirtualKeyRepo) FindByHash(ctx context.Context, keyHash string) (*models.VirtualKey, error) {
	key := &models.VirtualKey{}
	if err := r.db.WithContext(ctx).First(key, "key_hash = ? AND status = ?", keyHash, models.VirtualKeyStatusActive).Error; err != nil {
		return nil, fmt.Errorf("find virtual key: %w", mapError(err))
	}
	return key, nil
}

func (r *VirtualKeyRepo) GetByID(ctx context.Context, id string) (*models.VirtualKey, error) {
	key := &models.VirtualKey{}
	if err := r.db.WithContext(ctx).First(key, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get virtual key: %w", mapError(err))
	}
	return key, nil
}

func (r *VirtualKeyRepo) ListByOrg(ctx context.Context, orgID string) ([]models.VirtualKey, error) {
	var keys []models.VirtualKey
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list virtual keys: %w", err)
	}
	return keys, nil
}

func (r *VirtualKeyRepo) ListByProject(ctx context.Context, projectID string) ([]models.VirtualKey, error) {
	var keys []models.VirtualKey
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list project virtual keys: %w", err)
	}
	return keys, nil
}

func (r *VirtualKeyRepo) IncrementBudgetUsed(ctx context.Context, id string, amount float64) error {
	result := r.db.WithContext(ctx).Model(&models.VirtualKey{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"budget_used_usd": gorm.Expr("budget_used_usd + ?", amount),
			"updated_at":      time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("increment virtual key budget: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("increment virtual key budget: %w", ErrNotFound)
	}
	return nil
}

func (r *VirtualKeyRepo) Update(ctx context.Context, id string, input models.UpdateVirtualKeyInput) (*models.VirtualKey, error) {
	key, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now()}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.BudgetLimitUSD != nil {
		updates["budget_limit_usd"] = *input.BudgetLimitUSD
	}
	if input.RPMLimit != nil {
		updates["rpm_limit"] = *input.RPMLimit
	}
	if input.TPMLimit != nil {
		updates["tpm_limit"] = *input.TPMLimit
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.ExpiresAt != nil {
		updates["expires_at"] = *input.ExpiresAt
	}
	if err := r.db.WithContext(ctx).Model(key).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update virtual key: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *VirtualKeyRepo) Revoke(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&models.VirtualKey{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": models.VirtualKeyStatusRevoked, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("revoke virtual key: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("revoke virtual key: %w", ErrNotFound)
	}
	return nil
}

func (r *VirtualKeyRepo) Delete(ctx context.Context, id string) error {
	return r.Revoke(ctx, id)
}
