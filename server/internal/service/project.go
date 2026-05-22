package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type ProjectService struct{ repo *repository.ProjectRepo }

func NewProjectService(repo *repository.ProjectRepo) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) Create(ctx context.Context, orgID string, input models.CreateProjectInput) (*models.Project, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	return s.repo.Create(ctx, orgID, input)
}

func (s *ProjectService) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectService) ListByOrgID(ctx context.Context, orgID string) ([]models.Project, error) {
	return s.repo.ListByOrgID(ctx, orgID)
}

func (s *ProjectService) Update(ctx context.Context, id string, input models.UpdateProjectInput) (*models.Project, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *ProjectService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
