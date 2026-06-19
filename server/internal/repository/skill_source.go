package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type SkillSourceRepo struct{ db *gorm.DB }

func NewSkillSourceRepo(db *gorm.DB) *SkillSourceRepo {
	return &SkillSourceRepo{db: db}
}

func (r *SkillSourceRepo) Create(ctx context.Context, input models.CreateSkillSourceInput) (*models.SkillSource, error) {
	s := &models.SkillSource{URL: input.URL, Status: "pending"}
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return nil, fmt.Errorf("create skill source: %w", err)
	}
	return s, nil
}

func (r *SkillSourceRepo) GetByID(ctx context.Context, id string) (*models.SkillSource, error) {
	s := &models.SkillSource{}
	if err := r.db.WithContext(ctx).First(s, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get skill source: %w", mapError(err))
	}
	return s, nil
}

func (r *SkillSourceRepo) List(ctx context.Context) ([]models.SkillSource, error) {
	var list []models.SkillSource
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list skill sources: %w", err)
	}
	return list, nil
}

func (r *SkillSourceRepo) Update(ctx context.Context, id string, input models.UpdateSkillSourceInput) (*models.SkillSource, error) {
	s, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now()}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Error != nil {
		updates["error"] = *input.Error
	}
	if input.LastSyncedAt != nil {
		updates["last_synced_at"] = *input.LastSyncedAt
	}
	if err := r.db.WithContext(ctx).Model(s).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update skill source: %w", err)
	}
	return s, nil
}

func (r *SkillSourceRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.SkillSource{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete skill source: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete skill source: %w", ErrNotFound)
	}
	return nil
}
