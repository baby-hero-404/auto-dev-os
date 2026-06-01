package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type ProjectRepo struct{ db *gorm.DB }

func NewProjectRepo(db *gorm.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) Create(ctx context.Context, orgID string, input models.CreateProjectInput) (*models.Project, error) {
	p := &models.Project{OrgID: orgID, Name: input.Name, Description: input.Description}
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}

func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	p := &models.Project{}
	if err := r.db.WithContext(ctx).First(p, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get project: %w", mapError(err))
	}
	return p, nil
}

func (r *ProjectRepo) ListByOrgID(ctx context.Context, orgID string) ([]models.Project, error) {
	var projects []models.Project
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&projects).Error; err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return projects, nil
}

func (r *ProjectRepo) Update(ctx context.Context, id string, input models.UpdateProjectInput) (*models.Project, error) {
	p, err := r.GetByID(ctx, id)
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
	if err := r.db.WithContext(ctx).Model(p).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return p, nil
}

func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Project{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete project: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete project: %w", ErrNotFound)
	}
	return nil
}
