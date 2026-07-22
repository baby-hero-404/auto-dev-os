package repository

import (
	"context"
	"encoding/json"
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
	if input.DefaultModelLevel != nil {
		p.DefaultModelLevel = *input.DefaultModelLevel
	}
	if input.DefaultAutonomy != nil {
		p.DefaultAutonomy = *input.DefaultAutonomy
	}
	if input.AutoReviewPolicy != nil {
		p.AutoReviewPolicy = *input.AutoReviewPolicy
	}
	if input.MaxRetries != nil {
		p.MaxRetries = *input.MaxRetries
	}
	if input.MaxReviewFixCycles != nil {
		p.MaxReviewFixCycles = *input.MaxReviewFixCycles
	}
	if input.DefaultBranch != nil {
		p.DefaultBranch = *input.DefaultBranch
	}
	if input.ExecutionEngine != nil {
		p.ExecutionEngine = *input.ExecutionEngine
	}
	if input.CLIEngineConfig != nil {
		if raw, err := json.Marshal(*input.CLIEngineConfig); err == nil {
			p.CLIEngineConfig = raw
		}
	}
	if input.ReviewHarnessPolicy != nil {
		p.ReviewHarnessPolicy = *input.ReviewHarnessPolicy
	}
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
	if err := r.db.WithContext(ctx).
		Model(&models.Project{}).
		Select(`
			projects.*,
			(SELECT COUNT(*) FROM repositories WHERE repositories.project_id = projects.id) AS repositories_count,
			(SELECT COUNT(*) FROM agents WHERE agents.org_id = projects.org_id AND (agents.assignment_strategy = 'auto_join' OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = projects.id))) AS agents_count,
			(SELECT COUNT(*) FROM tasks WHERE tasks.project_id = projects.id AND tasks.status IN ('done', 'completed', 'merged')) AS tasks_done_count,
			(SELECT COUNT(*) FROM tasks WHERE tasks.project_id = projects.id) AS tasks_total_count
		`).
		Where("projects.org_id = ?", orgID).
		Order("projects.created_at DESC").
		Find(&projects).Error; err != nil {
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
	if input.DefaultModelLevel != nil {
		updates["default_model_level"] = *input.DefaultModelLevel
	}
	if input.DefaultAutonomy != nil {
		updates["default_autonomy"] = *input.DefaultAutonomy
	}
	if input.AutoReviewPolicy != nil {
		updates["auto_review_policy"] = *input.AutoReviewPolicy
	}
	if input.MaxRetries != nil {
		updates["max_retries"] = *input.MaxRetries
	}
	if input.MaxReviewFixCycles != nil {
		updates["max_review_fix_cycles"] = *input.MaxReviewFixCycles
	}
	if input.DefaultBranch != nil {
		updates["default_branch"] = *input.DefaultBranch
	}
	if input.ExecutionEngine != nil {
		updates["execution_engine"] = *input.ExecutionEngine
	}
	if input.CLIEngineConfig != nil {
		merged := *input.CLIEngineConfig
		// "***" in an env value means "keep the existing secret" (client echoes masked values back).
		if len(merged.Env) > 0 {
			var existing models.CLIEngineConfig
			_ = json.Unmarshal(p.CLIEngineConfig, &existing)
			for k, v := range merged.Env {
				if v == "***" {
					if old, ok := existing.Env[k]; ok {
						merged.Env[k] = old
					} else {
						delete(merged.Env, k)
					}
				}
			}
		}
		if raw, err := json.Marshal(merged); err == nil {
			updates["cli_engine_config"] = raw
		}
	}
	if input.ReviewHarnessPolicy != nil {
		updates["review_harness_policy"] = *input.ReviewHarnessPolicy
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
