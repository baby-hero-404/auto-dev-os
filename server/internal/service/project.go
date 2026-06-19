package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type ProjectService struct {
	repo     *repository.ProjectRepo
	seeder   *SeederService
	dataRoot string
}

func NewProjectService(repo *repository.ProjectRepo, seeder *SeederService, dataRoot string) *ProjectService {
	return &ProjectService{repo: repo, seeder: seeder, dataRoot: dataRoot}
}

func (s *ProjectService) Create(ctx context.Context, orgID string, input models.CreateProjectInput) (*models.Project, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	project, err := s.repo.Create(ctx, orgID, input)
	if err != nil {
		return nil, err
	}

	// Create project-specific directory structure on disk if dataRoot is set
	if s.dataRoot != "" {
		projDir := filepath.Join(s.dataRoot, "projects", project.ID)
		subdirs := []string{"rules", "skills", "docs"}
		for _, sub := range subdirs {
			dir := filepath.Join(projDir, sub)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create project directory %s: %w", sub, err)
			}
		}
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
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	// Clean up project-specific directory on disk
	if s.dataRoot != "" {
		projDir := filepath.Join(s.dataRoot, "projects", id)
		_ = os.RemoveAll(projDir)
	}

	return nil
}
