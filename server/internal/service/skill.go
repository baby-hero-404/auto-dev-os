package service

import (
	"context"
	"encoding/json"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type SkillService struct {
	repo       *repository.SkillRepo
	skillsRoot string
}

func NewSkillService(repo *repository.SkillRepo, skillsRoot string) *SkillService {
	return &SkillService{repo: repo, skillsRoot: skillsRoot}
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

func (s *SkillService) Test(ctx context.Context, id string, input map[string]any) (map[string]any, error) {
	skill, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"skill_id": skill.ID,
		"name":     skill.Name,
		"dry_run":  true,
		"input":    input,
		"schema":   json.RawMessage(skill.Schema),
	}, nil
}

func (s *SkillService) Update(ctx context.Context, id string, input models.UpdateSkillInput) (*models.Skill, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *SkillService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *SkillService) SeedDefaultSkills(ctx context.Context) ([]models.Skill, error) {
	defaults, err := loadPromptBaseSkills(s.skillsRoot)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(defaults))
	for i, d := range defaults {
		names[i] = d.Name
	}
	existing, err := s.repo.ListByNames(ctx, names)
	if err != nil {
		return nil, err
	}
	existingNames := make(map[string]bool)
	for _, skill := range existing {
		existingNames[skill.Name] = true
	}

	var created []models.Skill
	for _, input := range defaults {
		if existingNames[input.Name] {
			continue
		}
		skill, err := s.repo.Create(ctx, input)
		if err != nil {
			continue
		}
		created = append(created, *skill)
	}
	return created, nil
}
