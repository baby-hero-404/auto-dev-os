package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// AnalyticsDashboardRepo handles aggregation queries for the Phase 5 analytics dashboard.
type AnalyticsDashboardRepo struct{ db *gorm.DB }

func NewAnalyticsDashboardRepo(db *gorm.DB) *AnalyticsDashboardRepo {
	return &AnalyticsDashboardRepo{db: db}
}

// Overview returns platform-wide summary statistics.
func (r *AnalyticsDashboardRepo) Overview(ctx context.Context, orgID string) (*models.OverviewStats, error) {
	var stats models.OverviewStats
	db := r.db.WithContext(ctx)

	// Total projects.
	if err := db.Table("projects").Count(&stats.TotalProjects).Error; err != nil {
		return nil, fmt.Errorf("count projects: %w", err)
	}

	// Task counts.
	if err := db.Table("tasks").Count(&stats.TotalTasks).Error; err != nil {
		return nil, fmt.Errorf("count tasks: %w", err)
	}
	activeStatuses := []string{
		models.TaskStatusAnalyzing, models.TaskStatusAssigned,
		models.TaskStatusPlanning, models.TaskStatusCoding,
		models.TaskStatusReviewing, models.TaskStatusFixing,
		models.TaskStatusTesting, models.TaskStatusInProgress,
	}
	if err := db.Table("tasks").Where("status IN ?", activeStatuses).Count(&stats.ActiveTasks).Error; err != nil {
		return nil, fmt.Errorf("count active tasks: %w", err)
	}
	if err := db.Table("tasks").Where("status IN ?", []string{models.TaskStatusCompleted, models.TaskStatusMerged}).Count(&stats.CompletedTasks).Error; err != nil {
		return nil, fmt.Errorf("count completed tasks: %w", err)
	}
	if err := db.Table("tasks").Where("status = ?", models.TaskStatusFailed).Count(&stats.FailedTasks).Error; err != nil {
		return nil, fmt.Errorf("count failed tasks: %w", err)
	}

	// Success rate.
	finished := stats.CompletedTasks + stats.FailedTasks
	if finished > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(finished) * 100.0
	}

	// Agent counts.
	if err := db.Table("agents").Count(&stats.TotalAgents).Error; err != nil {
		return nil, fmt.Errorf("count agents: %w", err)
	}
	if err := db.Table("agents").Where("status IN ?", []string{models.AgentStatusBusy, models.AgentStatusRunning}).Count(&stats.RunningAgents).Error; err != nil {
		return nil, fmt.Errorf("count running agents: %w", err)
	}

	// Average completion time (from tasks that reached completed/merged status).
	if err := db.Table("tasks").
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000), 0)").
		Where("status IN ?", []string{models.TaskStatusCompleted, models.TaskStatusMerged}).
		Scan(&stats.AvgCompletionMs).Error; err != nil {
		return nil, fmt.Errorf("avg completion time: %w", err)
	}

	// Open PRs (tasks in human_review status).
	if err := db.Table("tasks").Where("status = ?", models.TaskStatusHumanReview).Count(&stats.OpenPRs).Error; err != nil {
		return nil, fmt.Errorf("count open prs: %w", err)
	}

	// Token cost summary.
	var tokenAgg struct {
		TotalCost   float64
		TotalTokens int64
	}
	if err := db.Table("token_usage").
		Select("COALESCE(SUM(cost_usd), 0) AS total_cost, COALESCE(SUM(prompt_tokens + output_tokens), 0) AS total_tokens").
		Scan(&tokenAgg).Error; err != nil {
		return nil, fmt.Errorf("token cost summary: %w", err)
	}
	stats.TotalTokenCost = tokenAgg.TotalCost
	stats.TotalTokensUsed = tokenAgg.TotalTokens

	return &stats, nil
}

// AgentPerformance returns per-agent performance metrics.
func (r *AnalyticsDashboardRepo) AgentPerformance(ctx context.Context, projectID string) ([]models.AgentStats, error) {
	query := r.db.WithContext(ctx).
		Table("agents a").
		Select(`
			a.id AS agent_id,
			a.name AS agent_name,
			a.role,
			a.provider,
			a.model,
			a.status,
			COALESCE(t.task_count, 0) AS task_count,
			COALESCE(t.success_count, 0) AS success_count,
			COALESCE(t.fail_count, 0) AS fail_count,
			CASE WHEN COALESCE(t.task_count, 0) > 0
				THEN (COALESCE(t.success_count, 0)::float / t.task_count * 100)
				ELSE 0 END AS success_rate,
			COALESCE(w.retry_count, 0) AS retry_count,
			COALESCE(tu.total_tokens, 0) AS total_tokens,
			COALESCE(tu.total_cost_usd, 0) AS total_cost_usd
		`).
		Joins(`LEFT JOIN (
			SELECT agent_id,
				COUNT(*) AS task_count,
				COUNT(*) FILTER (WHERE status IN ('completed', 'merged')) AS success_count,
				COUNT(*) FILTER (WHERE status = 'failed') AS fail_count
			FROM tasks WHERE agent_id IS NOT NULL GROUP BY agent_id
		) t ON t.agent_id = a.id`).
		Joins(`LEFT JOIN (
			SELECT agent_id, COALESCE(SUM(attempts) - COUNT(*), 0) AS retry_count
			FROM workflow_jobs WHERE agent_id IS NOT NULL GROUP BY agent_id
		) w ON w.agent_id = a.id`).
		Joins(`LEFT JOIN (
			SELECT agent_id,
				COALESCE(SUM(prompt_tokens + output_tokens), 0) AS total_tokens,
				COALESCE(SUM(cost_usd), 0) AS total_cost_usd
			FROM token_usage WHERE agent_id IS NOT NULL GROUP BY agent_id
		) tu ON tu.agent_id = a.id`).
		Order("task_count DESC")

	if projectID != "" {
		query = query.Where("a.id IN (SELECT agent_id FROM project_agents WHERE project_id = ?)", projectID)
	}

	var stats []models.AgentStats
	if err := query.Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("agent performance: %w", err)
	}
	return stats, nil
}

// TaskAnalytics returns task status distribution and time-series throughput.
func (r *AnalyticsDashboardRepo) TaskAnalytics(ctx context.Context, projectID string, days int) (*models.TaskAnalytics, error) {
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
			COUNT(*) FILTER (WHERE status IN ('completed', 'merged')) AS completed,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed
		`).
		Where("created_at >= ?", since).
		Group("bucket").
		Order("bucket ASC")
	if projectID != "" {
		tsQuery = tsQuery.Where("project_id = ?", projectID)
	}
	if err := tsQuery.Scan(&result.TimeSeries).Error; err != nil {
		return nil, fmt.Errorf("task time series: %w", err)
	}

	return result, nil
}

// WorkflowAnalytics returns workflow completion rates and step durations.
func (r *AnalyticsDashboardRepo) WorkflowAnalytics(ctx context.Context, projectID string) (*models.WorkflowAnalytics, error) {
	result := &models.WorkflowAnalytics{}
	db := r.db.WithContext(ctx)

	jobQuery := db.Table("workflow_jobs")
	if projectID != "" {
		jobQuery = jobQuery.Where("task_id IN (SELECT id FROM tasks WHERE project_id = ?)", projectID)
	}

	// Total / completed / failed counts.
	if err := jobQuery.Count(&result.TotalWorkflows).Error; err != nil {
		return nil, fmt.Errorf("count workflows: %w", err)
	}
	if err := jobQuery.Where("status = ?", models.WorkflowJobStatusDone).Count(&result.CompletedCount).Error; err != nil {
		return nil, fmt.Errorf("count completed workflows: %w", err)
	}
	if err := jobQuery.Where("status = ?", models.WorkflowJobStatusFailed).Count(&result.FailedCount).Error; err != nil {
		return nil, fmt.Errorf("count failed workflows: %w", err)
	}
	if result.TotalWorkflows > 0 {
		result.CompletionRate = float64(result.CompletedCount) / float64(result.TotalWorkflows) * 100.0
	}

	// Average duration (for completed workflows).
	if err := db.Table("workflow_jobs").
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

	if err := stepQuery.Scan(&result.StepStats).Error; err != nil {
		return nil, fmt.Errorf("step stats: %w", err)
	}

	return result, nil
}
