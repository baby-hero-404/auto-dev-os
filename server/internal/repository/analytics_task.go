package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TaskAnalytics returns task status distribution and time-series throughput.
func (r *AnalyticsDashboardRepo) TaskAnalytics(ctx context.Context, orgID string, projectID string, days int) (*models.TaskAnalytics, error) {
	result := &models.TaskAnalytics{}
	db := r.db.WithContext(ctx)

	// Status distribution.
	distQuery := db.Table("tasks").
		Select("status, COUNT(*) AS count").
		Group("status").
		Order("count DESC")
	if projectID != "" {
		distQuery = distQuery.Where("project_id = ?", projectID)
	}
	if orgID != "" {
		distQuery = distQuery.Where("project_id IN (SELECT id FROM projects WHERE org_id = ?)", orgID)
	}
	if err := distQuery.Scan(&result.Distribution).Error; err != nil {
		return nil, fmt.Errorf("task distribution: %w", err)
	}

	// Time-series (bucketed by day).
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)
	tsQuery := db.Table("tasks").
		Select(`
			DATE_TRUNC('day', created_at) AS bucket,
			COUNT(*) AS created,
			COUNT(*) FILTER (WHERE status = 'merged') AS completed,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed
		`).
		Where("created_at >= ?", since).
		Group("bucket").
		Order("bucket ASC")
	if projectID != "" {
		tsQuery = tsQuery.Where("project_id = ?", projectID)
	}
	if orgID != "" {
		tsQuery = tsQuery.Where("project_id IN (SELECT id FROM projects WHERE org_id = ?)", orgID)
	}
	if err := tsQuery.Scan(&result.TimeSeries).Error; err != nil {
		return nil, fmt.Errorf("task time series: %w", err)
	}

	return result, nil
}
