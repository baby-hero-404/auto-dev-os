package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type RoleTemplateRepo struct{ db *gorm.DB }

func NewRoleTemplateRepo(db *gorm.DB) *RoleTemplateRepo {
	return &RoleTemplateRepo{db: db}
}

func (r *RoleTemplateRepo) ListAll(ctx context.Context) ([]models.RoleTemplate, error) {
	var templates []models.RoleTemplate
	if err := r.db.WithContext(ctx).Order("role ASC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list role templates: %w", err)
	}
	return templates, nil
}

func (r *RoleTemplateRepo) GetByRole(ctx context.Context, role string) (*models.RoleTemplate, error) {
	template := &models.RoleTemplate{}
	if err := r.db.WithContext(ctx).First(template, "role = ?", role).Error; err != nil {
		return nil, fmt.Errorf("get role template: %w", mapError(err))
	}
	return template, nil
}
