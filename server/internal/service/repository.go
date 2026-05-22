package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type RepositoryService struct{ repo *repository.RepositoryRepo }

func NewRepositoryService(repo *repository.RepositoryRepo) *RepositoryService {
	return &RepositoryService{repo: repo}
}

func (s *RepositoryService) Create(ctx context.Context, projectID string, input models.CreateRepositoryInput) (*models.Repository, error) {
	if input.URL == "" {
		return nil, ErrValidation("url is required")
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *RepositoryService) GetByID(ctx context.Context, id string) (*models.Repository, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RepositoryService) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *RepositoryService) Update(ctx context.Context, id string, input models.UpdateRepositoryInput) (*models.Repository, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *RepositoryService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
