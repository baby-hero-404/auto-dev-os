package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type OrganizationRepo struct{ db *gorm.DB }

func NewOrganizationRepo(db *gorm.DB) *OrganizationRepo {
	return &OrganizationRepo{db: db}
}

func (r *OrganizationRepo) Create(ctx context.Context, input models.CreateOrganizationInput) (*models.Organization, error) {
	org := &models.Organization{Name: input.Name, Description: input.Description}
	if err := r.db.WithContext(ctx).Create(org).Error; err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	org := &models.Organization{}
	if err := r.db.WithContext(ctx).First(org, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get organization: %w", mapError(err))
	}
	return org, nil
}

func (r *OrganizationRepo) List(ctx context.Context) ([]models.Organization, error) {
	var orgs []models.Organization
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&orgs).Error; err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	return orgs, nil
}

func (r *OrganizationRepo) Update(ctx context.Context, id string, input models.UpdateOrganizationInput) (*models.Organization, error) {
	org, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if err := r.db.WithContext(ctx).Model(org).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Organization{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete organization: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete organization: %w", ErrNotFound)
	}
	return nil
}
