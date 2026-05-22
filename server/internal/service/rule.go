package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type RuleService struct{ repo *repository.RuleRepo }

func NewRuleService(repo *repository.RuleRepo) *RuleService {
	return &RuleService{repo: repo}
}

func (s *RuleService) Create(ctx context.Context, projectID *string, input models.CreateRuleInput) (*models.Rule, error) {
	if input.Content == "" {
		return nil, ErrValidation("content is required")
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *RuleService) GetByID(ctx context.Context, id string) (*models.Rule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RuleService) ListByProjectID(ctx context.Context, projectID string) ([]models.Rule, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *RuleService) Update(ctx context.Context, id string, input models.UpdateRuleInput) (*models.Rule, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *RuleService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
