package service

import (
	"context"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AnalyticsService struct {
	repo *repository.AnalyticsRepo
}

func NewAnalyticsService(repo *repository.AnalyticsRepo) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) TokenUsage(ctx context.Context, projectID string, since time.Time) ([]models.TokenUsageSummary, error) {
	return s.repo.TokenUsage(ctx, projectID, since)
}
