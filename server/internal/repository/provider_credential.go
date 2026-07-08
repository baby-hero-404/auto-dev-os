package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type ProviderCredentialRepo struct{ db *gorm.DB }

func NewProviderCredentialRepo(db *gorm.DB) *ProviderCredentialRepo {
	return &ProviderCredentialRepo{db: db}
}

func (r *ProviderCredentialRepo) Create(ctx context.Context, orgID string, input models.CreateProviderCredentialInput) (*models.ProviderCredential, error) {
	cred := &models.ProviderCredential{
		OrgID:        orgID,
		Provider:     input.Provider,
		Label:        input.Label,
		EncryptedKey: input.APIKey,
		BaseURL:      input.BaseURL,
		Status:       models.ProviderCredentialStatusActive,
		Priority:     input.Priority,
		Metadata:     input.Metadata,
	}
	if len(cred.Metadata) == 0 {
		cred.Metadata = []byte("{}")
	}
	if cred.Label == "" {
		cred.Label = "default"
	}
	if err := r.db.WithContext(ctx).Create(cred).Error; err != nil {
		return nil, fmt.Errorf("create provider credential: %w", mapError(err))
	}
	return cred, nil
}

func (r *ProviderCredentialRepo) GetByID(ctx context.Context, id string) (*models.ProviderCredential, error) {
	cred := &models.ProviderCredential{}
	if err := r.db.WithContext(ctx).First(cred, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get provider credential: %w", mapError(err))
	}
	return cred, nil
}

func (r *ProviderCredentialRepo) GetByIDAndOrg(ctx context.Context, orgID string, id string) (*models.ProviderCredential, error) {
	cred := &models.ProviderCredential{}
	if err := r.db.WithContext(ctx).First(cred, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return nil, fmt.Errorf("get provider credential: %w", mapError(err))
	}
	return cred, nil
}

func (r *ProviderCredentialRepo) ListByOrg(ctx context.Context, orgID string) ([]models.ProviderCredential, error) {
	var creds []models.ProviderCredential
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("provider ASC, priority ASC, created_at ASC").Find(&creds).Error; err != nil {
		return nil, fmt.Errorf("list provider credentials: %w", err)
	}
	return creds, nil
}

func (r *ProviderCredentialRepo) ListActiveByOrgAndProvider(ctx context.Context, orgID, provider string) ([]models.ProviderCredential, error) {
	var creds []models.ProviderCredential
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND provider = ? AND status = ?", orgID, provider, models.ProviderCredentialStatusActive).
		Where("cooldown_until IS NULL OR cooldown_until < NOW()").
		Order("priority ASC, created_at ASC").
		Find(&creds).Error
	if err != nil {
		return nil, fmt.Errorf("list active provider credentials: %w", err)
	}
	return creds, nil
}

func (r *ProviderCredentialRepo) Update(ctx context.Context, orgID string, id string, input models.UpdateProviderCredentialInput) (*models.ProviderCredential, error) {
	cred, err := r.GetByIDAndOrg(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now()}
	if input.Label != nil {
		updates["label"] = *input.Label
	}
	if input.APIKey != nil {
		updates["encrypted_key"] = *input.APIKey
	}
	if input.BaseURL != nil {
		updates["base_url"] = *input.BaseURL
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Priority != nil {
		updates["priority"] = *input.Priority
	}
	if input.Metadata != nil {
		updates["metadata"] = *input.Metadata
	}
	if err := r.db.WithContext(ctx).Model(cred).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update provider credential: %w", err)
	}
	return r.GetByIDAndOrg(ctx, orgID, id)
}

func (r *ProviderCredentialRepo) SetCooldown(ctx context.Context, id string, until time.Time) error {
	result := r.db.WithContext(ctx).Model(&models.ProviderCredential{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":         models.ProviderCredentialStatusRateLimited,
			"cooldown_until": until,
			"updated_at":     time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("set provider credential cooldown: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("set provider credential cooldown: %w", ErrNotFound)
	}
	return nil
}

func (r *ProviderCredentialRepo) ClearExpiredCooldowns(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ProviderCredential{}).
		Where("status = ? AND cooldown_until < NOW()", models.ProviderCredentialStatusRateLimited).
		Updates(map[string]any{
			"status":         models.ProviderCredentialStatusActive,
			"cooldown_until": nil,
			"updated_at":     time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("clear provider credential cooldowns: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *ProviderCredentialRepo) GetExpiredCooldowns(ctx context.Context) ([]models.ProviderCredential, error) {
	var creds []models.ProviderCredential
	err := r.db.WithContext(ctx).
		Where("status = ? AND cooldown_until < NOW()", models.ProviderCredentialStatusRateLimited).
		Find(&creds).Error
	if err != nil {
		return nil, fmt.Errorf("get expired cooldowns: %w", err)
	}
	return creds, nil
}

func (r *ProviderCredentialRepo) Delete(ctx context.Context, orgID string, id string) error {
	result := r.db.WithContext(ctx).Where("org_id = ?", orgID).Delete(&models.ProviderCredential{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete provider credential: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete provider credential: %w", ErrNotFound)
	}
	return nil
}
