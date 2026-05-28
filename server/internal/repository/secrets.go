package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type SecretRepo struct{ db *gorm.DB }

func NewSecretRepo(db *gorm.DB) *SecretRepo {
	return &SecretRepo{db: db}
}

func (r *SecretRepo) Upsert(ctx context.Context, projectID string, input models.CreateSecretInput) (*models.Secret, error) {
	secret := &models.Secret{}
	err := r.db.WithContext(ctx).Where("project_id = ? AND name = ?", projectID, input.Name).First(secret).Error
	if err == nil {
		secret.Value = input.Value
		if err := r.db.WithContext(ctx).Save(secret).Error; err != nil {
			return nil, fmt.Errorf("update secret: %w", err)
		}
		return secret, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("find secret: %w", err)
	}

	secret = &models.Secret{ProjectID: projectID, Name: input.Name, Value: input.Value}
	if err := r.db.WithContext(ctx).Create(secret).Error; err != nil {
		return nil, fmt.Errorf("create secret: %w", err)
	}
	return secret, nil
}

func (r *SecretRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Secret, error) {
	var secrets []models.Secret
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("name ASC").Find(&secrets).Error; err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	return secrets, nil
}
