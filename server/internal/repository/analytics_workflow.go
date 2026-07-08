package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// WorkflowAnalytics returns workflow completion rates and step durations.
func (r *AnalyticsDashboardRepo) WorkflowAnalytics(ctx context.Context, orgID string, projectID string) (*models.WorkflowAnalytics, error) {
	result := &models.WorkflowAnalytics{}
	db := r.db.WithContext(ctx)

	scopedJobs := func() *gorm.DB {
		query := db.Table("workflow_jobs")
		if projectID != "" {
			query = query.Where("task_id IN (SELECT id FROM tasks WHERE project_id = ?)", projectID)
		}
		if orgID != "" {
			query = query.Where("task_id IN (SELECT tasks.id FROM tasks JOIN projects ON projects.id = tasks.project_id WHERE projects.org_id = ?)", orgID)
		}
		return query
	}

	// Total / completed / failed counts.
	if err := scopedJobs().Count(&result.TotalWorkflows).Error; err != nil {
		return nil, fmt.Errorf("count workflows: %w", err)
	}
	if err := scopedJobs().Where("status = ?", models.WorkflowJobStatusDone).Count(&result.CompletedCount).Error; err != nil {
		return nil, fmt.Errorf("count completed workflows: %w", err)
	}
	if err := scopedJobs().Where("status = ?", models.WorkflowJobStatusFailed).Count(&result.FailedCount).Error; err != nil {
		return nil, fmt.Errorf("count failed workflows: %w", err)
	}
	if result.TotalWorkflows > 0 {
		result.CompletionRate = float64(result.CompletedCount) / float64(result.TotalWorkflows) * 100.0
	}

	// Average duration (for completed workflows).
	if err := scopedJobs().
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000), 0)").
		Where("status = ?", models.WorkflowJobStatusDone).
		Scan(&result.AvgDurationMs).Error; err != nil {
		return nil, fmt.Errorf("avg workflow duration: %w", err)
	}

	// Per-step stats from checkpoints.
	stepQuery := db.Table("workflow_checkpoints wc").
		Select(`
			wc.step,
			COALESCE(AVG(EXTRACT(EPOCH FROM (wc2.created_at - wc.created_at)) * 1000), 0) AS avg_ms,
			COUNT(*) AS total_runs,
			0 AS fail_count
		`).
		Joins(`LEFT JOIN workflow_checkpoints wc2 ON wc2.task_id = wc.task_id
			AND wc2.created_at > wc.created_at
			AND wc2.id = (
				SELECT id FROM workflow_checkpoints
				WHERE task_id = wc.task_id AND created_at > wc.created_at
				ORDER BY created_at ASC LIMIT 1
			)`).
		Group("wc.step").
		Order("total_runs DESC")
	if projectID != "" {
		stepQuery = stepQuery.Where("wc.task_id IN (SELECT id FROM tasks WHERE project_id = ?)", projectID)
	}
	if orgID != "" {
		stepQuery = stepQuery.Where("wc.task_id IN (SELECT tasks.id FROM tasks JOIN projects ON projects.id = tasks.project_id WHERE projects.org_id = ?)", orgID)
	}

	if err := stepQuery.Scan(&result.StepStats).Error; err != nil {
		return nil, fmt.Errorf("step stats: %w", err)
	}

	return result, nil
}
