package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type SkillService struct{ repo *repository.SkillRepo }

func NewSkillService(repo *repository.SkillRepo) *SkillService {
	return &SkillService{repo: repo}
}

func (s *SkillService) Create(ctx context.Context, input models.CreateSkillInput) (*models.Skill, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	return s.repo.Create(ctx, input)
}

func (s *SkillService) GetByID(ctx context.Context, id string) (*models.Skill, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SkillService) List(ctx context.Context) ([]models.Skill, error) {
	return s.repo.List(ctx)
}

func (s *SkillService) Update(ctx context.Context, id string, input models.UpdateSkillInput) (*models.Skill, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *SkillService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
