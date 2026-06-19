package service

import (
	"context"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type ProviderModelService struct {
	repo *repository.ProviderModelRepo
}

func NewProviderModelService(repo *repository.ProviderModelRepo) *ProviderModelService {
	return &ProviderModelService{repo: repo}
}

func (s *ProviderModelService) Create(ctx context.Context, orgID string, input models.CreateProviderModelInput) (*models.ProviderModel, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		return nil, ErrValidation("provider is required")
	}
	if provider != "openai" && provider != "anthropic" && provider != "gemini" && provider != "9router" {
		return nil, ErrValidation("unsupported provider: must be openai, anthropic, gemini, or 9router")
	}

	levelGroup := strings.ToLower(strings.TrimSpace(input.LevelGroup))
	if levelGroup == "" {
		return nil, ErrValidation("level_group is required")
	}
	if levelGroup != models.ModelLevelFast && levelGroup != models.ModelLevelBalanced && levelGroup != models.ModelLevelPowerful {
		return nil, ErrValidation("level_group must be fast, balanced, or powerful")
	}

	modelName := strings.TrimSpace(input.ModelName)
	if modelName == "" {
		return nil, ErrValidation("model_name is required")
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	input.Provider = provider
	input.LevelGroup = levelGroup
	input.ModelName = modelName
	input.IsActive = &isActive

	return s.repo.Create(ctx, orgID, input)
}

func (s *ProviderModelService) ListByOrg(ctx context.Context, orgID string, filter models.ProviderModelFilter) ([]models.ProviderModel, error) {
	if filter.Provider != nil {
		p := strings.ToLower(strings.TrimSpace(*filter.Provider))
		filter.Provider = &p
	}
	if filter.LevelGroup != nil {
		l := strings.ToLower(strings.TrimSpace(*filter.LevelGroup))
		filter.LevelGroup = &l
	}
	return s.repo.ListByOrg(ctx, orgID, filter)
}

func (s *ProviderModelService) ResolveModels(ctx context.Context, orgID string, levelGroup string) ([]models.ProviderModel, error) {
	levelGroup = strings.ToLower(strings.TrimSpace(levelGroup))
	if levelGroup == "" {
		levelGroup = models.ModelLevelBalanced
	}

	filter := models.ProviderModelFilter{
		LevelGroup: &levelGroup,
	}

	modelsList, err := s.repo.ListByOrg(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}

	var activeModels []models.ProviderModel
	for _, m := range modelsList {
		if m.IsActive {
			activeModels = append(activeModels, m)
		}
	}
	return activeModels, nil
}

func (s *ProviderModelService) Update(ctx context.Context, orgID string, id string, input models.UpdateProviderModelInput) (*models.ProviderModel, error) {
	if input.Provider != nil {
		p := strings.ToLower(strings.TrimSpace(*input.Provider))
		if p == "" {
			return nil, ErrValidation("provider cannot be empty")
		}
		if p != "openai" && p != "anthropic" && p != "gemini" && p != "9router" {
			return nil, ErrValidation("unsupported provider: must be openai, anthropic, gemini, or 9router")
		}
		input.Provider = &p
	}

	if input.LevelGroup != nil {
		l := strings.ToLower(strings.TrimSpace(*input.LevelGroup))
		if l == "" {
			return nil, ErrValidation("level_group cannot be empty")
		}
		if l != models.ModelLevelFast && l != models.ModelLevelBalanced && l != models.ModelLevelPowerful {
			return nil, ErrValidation("level_group must be fast, balanced, or powerful")
		}
		input.LevelGroup = &l
	}

	if input.ModelName != nil {
		m := strings.TrimSpace(*input.ModelName)
		if m == "" {
			return nil, ErrValidation("model_name cannot be empty")
		}
		input.ModelName = &m
	}

	return s.repo.Update(ctx, orgID, id, input)
}

func (s *ProviderModelService) Delete(ctx context.Context, orgID string, id string) error {
	return s.repo.Delete(ctx, orgID, id)
}
