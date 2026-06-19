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
func (s *AnalyticsDashboardService) AgentPerformance(ctx context.Context, orgID string, projectID string) ([]models.AgentStats, error) {
	return s.repo.AgentPerformance(ctx, orgID, projectID)
}

// TaskAnalytics returns task status distribution and time-series throughput.
func (s *AnalyticsDashboardService) TaskAnalytics(ctx context.Context, orgID string, projectID string, days int) (*models.TaskAnalytics, error) {
	return s.repo.TaskAnalytics(ctx, orgID, projectID, days)
}

// GatewayUsage returns daily gateway request, token, cost, and latency aggregates.
func (s *AnalyticsDashboardService) GatewayUsage(ctx context.Context, orgID string, projectID string, days int) ([]models.GatewayUsagePoint, error) {
	return s.repo.GatewayUsage(ctx, orgID, projectID, days)
}

// WorkflowAnalytics returns workflow completion rates and average step durations.
func (s *AnalyticsDashboardService) WorkflowAnalytics(ctx context.Context, orgID string, projectID string) (*models.WorkflowAnalytics, error) {
	return s.repo.WorkflowAnalytics(ctx, orgID, projectID)
}

// RecentFailures returns the latest failed tasks with workflow error context.
func (s *AnalyticsDashboardService) RecentFailures(ctx context.Context, orgID string, projectID string, limit int) ([]models.RecentFailure, error) {
	return s.repo.RecentFailures(ctx, orgID, projectID, limit)
}
