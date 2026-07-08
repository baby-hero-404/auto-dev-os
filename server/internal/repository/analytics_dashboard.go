package repository

import (
	"context"
	"fmt"

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

	projQuery := db.Table("projects")
	if orgID != "" {
		projQuery = projQuery.Where("org_id = ?", orgID)
	}

	scopedTaskQuery := func() *gorm.DB {
		query := db.Table("tasks")
		if orgID != "" {
			query = query.Where("project_id IN (SELECT id FROM projects WHERE org_id = ?)", orgID)
		}
		return query
	}

	agentQuery := db.Table("agents")
	if orgID != "" {
		agentQuery = agentQuery.Where("org_id = ?", orgID)
	}

	tokenQuery := db.Table("token_usage")
	if orgID != "" {
		tokenQuery = tokenQuery.Where("org_id = ?", orgID)
	}

	// Total projects.
	if err := projQuery.Count(&stats.TotalProjects).Error; err != nil {
		return nil, fmt.Errorf("count projects: %w", err)
	}

	// Task counts.
	if err := scopedTaskQuery().Count(&stats.TotalTasks).Error; err != nil {
		return nil, fmt.Errorf("count tasks: %w", err)
	}
	activeStatuses := []string{
		models.TaskStatusAnalyzing, models.TaskStatusCoding,
		models.TaskStatusReviewing, models.TaskStatusFixing,
		models.TaskStatusTesting, models.TaskStatusHumanReview,
	}
	if err := scopedTaskQuery().Where("status IN ?", activeStatuses).Count(&stats.ActiveTasks).Error; err != nil {
		return nil, fmt.Errorf("count active tasks: %w", err)
	}
	if err := scopedTaskQuery().Where("status = ?", models.TaskStatusMerged).Count(&stats.CompletedTasks).Error; err != nil {
		return nil, fmt.Errorf("count completed tasks: %w", err)
	}
	if err := scopedTaskQuery().Where("status = ?", models.TaskStatusFailed).Count(&stats.FailedTasks).Error; err != nil {
		return nil, fmt.Errorf("count failed tasks: %w", err)
	}

	// Success rate.
	finished := stats.CompletedTasks + stats.FailedTasks
	if finished > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(finished) * 100.0
	}

	// Agent counts.
	if err := agentQuery.Count(&stats.TotalAgents).Error; err != nil {
		return nil, fmt.Errorf("count agents: %w", err)
	}
	if err := agentQuery.Where("status IN ?", []string{models.AgentStatusBusy, models.AgentStatusRunning}).Count(&stats.RunningAgents).Error; err != nil {
		return nil, fmt.Errorf("count running agents: %w", err)
	}

	// Average completion time (from tasks that reached completed/merged status).
	if err := scopedTaskQuery().
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000), 0)").
		Where("status = ?", models.TaskStatusMerged).
		Scan(&stats.AvgCompletionMs).Error; err != nil {
		return nil, fmt.Errorf("avg completion time: %w", err)
	}

	// Open PRs (tasks in human_review status).
	if err := scopedTaskQuery().Where("status = ?", models.TaskStatusHumanReview).Count(&stats.OpenPRs).Error; err != nil {
		return nil, fmt.Errorf("count open prs: %w", err)
	}

	// Token cost summary.
	var tokenAgg struct {
		TotalCost   float64
		TotalTokens int64
	}
	if err := tokenQuery.
		Select("COALESCE(SUM(cost_usd), 0) AS total_cost, COALESCE(SUM(prompt_tokens + output_tokens), 0) AS total_tokens").
		Scan(&tokenAgg).Error; err != nil {
		return nil, fmt.Errorf("token cost summary: %w", err)
	}
	stats.TotalTokenCost = tokenAgg.TotalCost
	stats.TotalTokensUsed = tokenAgg.TotalTokens

	return &stats, nil
}

// RecentFailures returns the latest failed tasks with workflow error context.
func (r *AnalyticsDashboardRepo) RecentFailures(ctx context.Context, orgID string, projectID string, limit int) ([]models.RecentFailure, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	query := r.db.WithContext(ctx).
		Table("tasks t").
		Select(`
			t.id AS task_id,
			t.project_id,
			p.name AS project_name,
			t.title,
			COALESCE((
				SELECT last_error FROM workflow_jobs
				WHERE task_id = t.id AND last_error <> ''
				ORDER BY updated_at DESC LIMIT 1
			), '') AS failure_reason,
			COALESCE((
				SELECT step FROM workflow_jobs
				WHERE task_id = t.id
				ORDER BY updated_at DESC LIMIT 1
			), '') AS workflow_step,
			t.updated_at AS failed_at
		`).
		Joins("JOIN projects p ON p.id = t.project_id").
		Where("t.status = ?", models.TaskStatusFailed).
		Order("t.updated_at DESC").
		Limit(limit)

	if orgID != "" {
		query = query.Where("p.org_id = ?", orgID)
	}
	if projectID != "" {
		query = query.Where("t.project_id = ?", projectID)
	}

	var failures []models.RecentFailure
	if err := query.Scan(&failures).Error; err != nil {
		return nil, fmt.Errorf("recent failures: %w", err)
	}
	return failures, nil
}
