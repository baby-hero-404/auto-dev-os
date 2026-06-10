package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type ResolvedRoute struct {
	Entries []models.ComboEntry
}

type ModelRouteService struct {
	repo *repository.ModelRouteRepo
}

func NewModelRouteService(repo *repository.ModelRouteRepo) *ModelRouteService {
	return &ModelRouteService{repo: repo}
}

func (s *ModelRouteService) Create(ctx context.Context, orgID string, input models.CreateModelRouteInput) (*models.ModelRoute, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	if input.RouteType == "" {
		input.RouteType = models.ModelRouteTypeCombo
	}
	if len(input.Config) == 0 {
		return nil, ErrValidation("config is required")
	}
	return s.repo.Create(ctx, orgID, input)
}

func (s *ModelRouteService) ListByOrg(ctx context.Context, orgID string) ([]models.ModelRoute, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *ModelRouteService) Update(ctx context.Context, id string, input models.UpdateModelRouteInput) (*models.ModelRoute, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *ModelRouteService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *ModelRouteService) ResolveRoute(ctx context.Context, orgID, routeName, complexity string) (*ResolvedRoute, error) {
	var route *models.ModelRoute
	var err error
	if routeName != "" {
		route, err = s.repo.GetByName(ctx, orgID, routeName)
	} else {
		route, err = s.repo.GetDefault(ctx, orgID)
	}
	if err != nil {
		return nil, err
	}
	var entries []models.ComboEntry
	if err := json.Unmarshal(route.Config, &entries); err != nil {
		return nil, ErrValidation("route config must be a combo entry array")
	}
	tier := tierForComplexity(complexity)
	filtered := make([]models.ComboEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Provider == "" || entry.Model == "" {
			continue
		}
		if entry.Tier == "" || entry.Tier == tier {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		filtered = entries
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Priority < filtered[j].Priority
	})
	return &ResolvedRoute{Entries: filtered}, nil
}

func tierForComplexity(complexity string) string {
	switch strings.ToLower(complexity) {
	case models.TaskComplexityEasy:
		return "fast"
	case models.TaskComplexityHard:
		return "powerful"
	default:
		return "balanced"
	}
}
