package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type ProjectService struct {
	repo   *repository.ProjectRepo
	seeder *SeederService
}

func NewProjectService(repo *repository.ProjectRepo, seeder *SeederService) *ProjectService {
	return &ProjectService{repo: repo, seeder: seeder}
}

func (s *ProjectService) Create(ctx context.Context, orgID string, input models.CreateProjectInput) (*models.Project, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	project, err := s.repo.Create(ctx, orgID, input)
	if err != nil {
		return nil, err
	}
	// Seed default rules and skills asynchronously so project creation stays fast.
	go s.seeder.SeedProject(context.Background(), project.ID)
	return project, nil
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
