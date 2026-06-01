package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// AnalyticsDashboardService provides business logic for the Phase 5 analytics dashboard.
type AnalyticsDashboardService struct {
	repo *repository.AnalyticsDashboardRepo
}

func NewAnalyticsDashboardService(repo *repository.AnalyticsDashboardRepo) *AnalyticsDashboardService {
	return &AnalyticsDashboardService{repo: repo}
}

// Overview returns high-level platform statistics.
func (s *AnalyticsDashboardService) Overview(ctx context.Context, orgID string) (*models.OverviewStats, error) {
	return s.repo.Overview(ctx, orgID)
}

// AgentPerformance returns per-agent performance metrics.
func (s *AnalyticsDashboardService) AgentPerformance(ctx context.Context, projectID string) ([]models.AgentStats, error) {
	return s.repo.AgentPerformance(ctx, projectID)
}

// TaskAnalytics returns task status distribution and time-series throughput.
func (s *AnalyticsDashboardService) TaskAnalytics(ctx context.Context, projectID string, days int) (*models.TaskAnalytics, error) {
	return s.repo.TaskAnalytics(ctx, projectID, days)
}

// WorkflowAnalytics returns workflow completion rates and average step durations.
func (s *AnalyticsDashboardService) WorkflowAnalytics(ctx context.Context, projectID string) (*models.WorkflowAnalytics, error) {
	return s.repo.WorkflowAnalytics(ctx, projectID)
}
