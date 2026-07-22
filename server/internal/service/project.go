package service

import (
	"context"
	"encoding/json"
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
	if err := validateEngineInput(input.ExecutionEngine, input.CLIEngineConfig); err != nil {
		return nil, err
	}
	if input.ReviewHarnessPolicy != nil {
		if err := models.ValidateReviewHarnessPolicy(*input.ReviewHarnessPolicy); err != nil {
			return nil, ErrValidation(err.Error())
		}
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
	project, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	maskProjectCLIEnv(project)
	return project, nil
}

func (s *ProjectService) ListByOrgID(ctx context.Context, orgID string) ([]models.Project, error) {
	projects, err := s.repo.ListByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range projects {
		maskProjectCLIEnv(&projects[i])
	}
	return projects, nil
}

func (s *ProjectService) Update(ctx context.Context, id string, input models.UpdateProjectInput) (*models.Project, error) {
	engine := ""
	if input.ExecutionEngine != nil {
		engine = *input.ExecutionEngine
	}
	if err := validateEngineInput(input.ExecutionEngine, input.CLIEngineConfig); err != nil {
		return nil, err
	}
	if input.ReviewHarnessPolicy != nil {
		if err := models.ValidateReviewHarnessPolicy(*input.ReviewHarnessPolicy); err != nil {
			return nil, ErrValidation(err.Error())
		}
	}
	if engine == models.ExecutionEngineCLI && input.CLIEngineConfig == nil {
		// engine flipping to cli without a config payload in this PATCH: verify an existing config has a command.
		existing, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		var cfg models.CLIEngineConfig
		_ = json.Unmarshal(existing.CLIEngineConfig, &cfg)
		if err := models.ValidateCLIEngineConfig(engine, &cfg); err != nil {
			return nil, ErrValidation(err.Error())
		}
	}
	project, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	maskProjectCLIEnv(project)
	return project, nil
}

func validateEngineInput(engine *string, cfg *models.CLIEngineConfig) error {
	if engine != nil {
		if err := models.ValidateExecutionEngine(*engine); err != nil {
			return ErrValidation(err.Error())
		}
		if *engine == models.ExecutionEngineCLI && cfg != nil {
			if err := models.ValidateCLIEngineConfig(*engine, cfg); err != nil {
				return ErrValidation(err.Error())
			}
		}
	}
	return nil
}

func maskProjectCLIEnv(p *models.Project) {
	if p == nil || len(p.CLIEngineConfig) == 0 {
		return
	}
	var cfg models.CLIEngineConfig
	if json.Unmarshal(p.CLIEngineConfig, &cfg) != nil {
		return
	}
	masked := cfg.MaskedEnv()
	if raw, err := json.Marshal(masked); err == nil {
		p.CLIEngineConfig = raw
	}
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
