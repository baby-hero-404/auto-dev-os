package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/governance"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// validatePipelineConfig runs governance.ValidateConfig (schema + DAG
// checks, REQ-001/REQ-001b) and turns any failures into a single
// ErrValidation with all violations joined, since ErrValidation only
// carries a message string. A nil/empty raw is a no-op (REQ-002).
func validatePipelineConfig(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	_, errs, err := governance.ValidateConfig(raw)
	if err != nil {
		return ErrValidation(fmt.Sprintf("pipeline_config: %v", err))
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = fmt.Sprintf("%s: %s", e.Path, e.Message)
		}
		return ErrValidation("pipeline_config: " + strings.Join(msgs, "; "))
	}
	return nil
}

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
	if _, err := models.ValidateExecutionProviders(input.ExecutionProviders); err != nil {
		return nil, ErrValidation(err.Error())
	}
	if input.ReviewHarnessPolicy != nil {
		if err := models.ValidateReviewHarnessPolicy(*input.ReviewHarnessPolicy); err != nil {
			return nil, ErrValidation(err.Error())
		}
	}
	if err := validatePipelineConfig(input.PipelineConfig); err != nil {
		return nil, err
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
	if _, err := models.ValidateExecutionProviders(input.ExecutionProviders); err != nil {
		return nil, ErrValidation(err.Error())
	}
	if input.ReviewHarnessPolicy != nil {
		if err := models.ValidateReviewHarnessPolicy(*input.ReviewHarnessPolicy); err != nil {
			return nil, ErrValidation(err.Error())
		}
	}
	if err := validatePipelineConfig(input.PipelineConfig); err != nil {
		return nil, err
	}
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
