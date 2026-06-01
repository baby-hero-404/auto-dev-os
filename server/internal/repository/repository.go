package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type RepositoryRepo struct{ db *gorm.DB }

func NewRepositoryRepo(db *gorm.DB) *RepositoryRepo {
	return &RepositoryRepo{db: db}
}

func (r *RepositoryRepo) Create(ctx context.Context, projectID string, input models.CreateRepositoryInput) (*models.Repository, error) {
	repo := &models.Repository{
		ProjectID: projectID, URL: input.URL,
		Provider: input.Provider, Branch: input.Branch, Token: input.Token,
	}
	if err := r.db.WithContext(ctx).Create(repo).Error; err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}
	return repo, nil
}

func (r *RepositoryRepo) GetByID(ctx context.Context, id string) (*models.Repository, error) {
	repo := &models.Repository{}
	if err := r.db.WithContext(ctx).First(repo, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get repository: %w", mapError(err))
	}
	return repo, nil
}

func (r *RepositoryRepo) GetByURL(ctx context.Context, repoURL string) (*models.Repository, error) {
	repo := &models.Repository{}
	if err := r.db.WithContext(ctx).First(repo, "url = ?", repoURL).Error; err != nil {
		return nil, fmt.Errorf("get repository by url: %w", mapError(err))
	}
	return repo, nil
}

func (r *RepositoryRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	var repos []models.Repository
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	return repos, nil
}

func (r *RepositoryRepo) Update(ctx context.Context, id string, input models.UpdateRepositoryInput) (*models.Repository, error) {
	repo, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.URL != nil {
		updates["url"] = *input.URL
	}
	if input.Provider != nil {
		updates["provider"] = *input.Provider
	}
	if input.Branch != nil {
		updates["branch"] = *input.Branch
	}
	if input.Token != nil {
		updates["token"] = *input.Token
	}
	if input.ClonePath != nil {
		updates["clone_path"] = *input.ClonePath
	}
	if input.CloneStatus != nil {
		updates["clone_status"] = *input.CloneStatus
	}
	if err := r.db.WithContext(ctx).Model(repo).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update repository: %w", err)
	}
	return repo, nil
}

func (r *RepositoryRepo) MarkValidated(ctx context.Context, id string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&models.Repository{}).Where("id = ?", id).Update("last_validated_at", now).Error; err != nil {
		return fmt.Errorf("mark repository validated: %w", err)
	}
	return nil
}

func (r *RepositoryRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Repository{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete repository: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete repository: %w", ErrNotFound)
	}
	return nil
}
