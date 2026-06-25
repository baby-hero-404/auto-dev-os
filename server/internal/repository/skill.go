package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type SkillRepo struct{ db *gorm.DB }

func NewSkillRepo(db *gorm.DB) *SkillRepo {
	return &SkillRepo{db: db}
}

func (r *SkillRepo) Create(ctx context.Context, input models.CreateSkillInput) (*models.Skill, error) {
	s := &models.Skill{Name: input.Name, Description: input.Description, Schema: input.Schema}
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return nil, fmt.Errorf("create skill: %w", err)
	}
	return s, nil
}

func (r *SkillRepo) GetByID(ctx context.Context, id string) (*models.Skill, error) {
	s := &models.Skill{}
	if err := r.db.WithContext(ctx).First(s, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get skill: %w", mapError(err))
	}
	return s, nil
}

func (r *SkillRepo) List(ctx context.Context) ([]models.Skill, error) {
	var skills []models.Skill
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&skills).Error; err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	return skills, nil
}

func (r *SkillRepo) Update(ctx context.Context, id string, input models.UpdateSkillInput) (*models.Skill, error) {
	s, err := r.GetByID(ctx, id)
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
	if input.Schema != nil {
		updates["schema"] = *input.Schema
	}
	if err := r.db.WithContext(ctx).Model(s).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update skill: %w", err)
	}
	return s, nil
}

func (r *SkillRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Skill{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete skill: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete skill: %w", ErrNotFound)
	}
	return nil
}
