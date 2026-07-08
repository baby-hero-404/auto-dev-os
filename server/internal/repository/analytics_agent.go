package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// AgentPerformance returns per-agent performance metrics.
func (r *AnalyticsDashboardRepo) AgentPerformance(ctx context.Context, orgID string, projectID string) ([]models.AgentStats, error) {
	taskWhere := "agent_id IS NOT NULL"
	taskArgs := []any{}
	workflowWhere := "agent_id IS NOT NULL"
	workflowArgs := []any{}
	tokenWhere := "agent_id IS NOT NULL"
	tokenArgs := []any{}

	if orgID != "" {
		taskWhere += " AND project_id IN (SELECT id FROM projects WHERE org_id = ?)"
		taskArgs = append(taskArgs, orgID)
		workflowWhere += " AND task_id IN (SELECT tasks.id FROM tasks JOIN projects ON projects.id = tasks.project_id WHERE projects.org_id = ?)"
		workflowArgs = append(workflowArgs, orgID)
		tokenWhere += " AND org_id = ?"
		tokenArgs = append(tokenArgs, orgID)
	}
	if projectID != "" {
		taskWhere += " AND project_id = ?"
		taskArgs = append(taskArgs, projectID)
		workflowWhere += " AND task_id IN (SELECT id FROM tasks WHERE project_id = ?)"
		workflowArgs = append(workflowArgs, projectID)
		tokenWhere += " AND project_id = ?"
		tokenArgs = append(tokenArgs, projectID)
	}

	query := r.db.WithContext(ctx).
		Table("agents a").
		Select(`
			a.id AS agent_id,
			a.name AS agent_name,
			a.role,
			a.model_level_group,
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
				COUNT(*) FILTER (WHERE status = 'merged') AS success_count,
				COUNT(*) FILTER (WHERE status = 'failed') AS fail_count
			FROM tasks WHERE `+taskWhere+` GROUP BY agent_id
		) t ON t.agent_id = a.id`, taskArgs...).
		Joins(`LEFT JOIN (
			SELECT agent_id, COALESCE(SUM(attempts) - COUNT(*), 0) AS retry_count
			FROM workflow_jobs WHERE `+workflowWhere+` GROUP BY agent_id
		) w ON w.agent_id = a.id`, workflowArgs...).
		Joins(`LEFT JOIN (
			SELECT agent_id,
				COALESCE(SUM(prompt_tokens + output_tokens), 0) AS total_tokens,
				COALESCE(SUM(cost_usd), 0) AS total_cost_usd
			FROM token_usage WHERE `+tokenWhere+` GROUP BY agent_id
		) tu ON tu.agent_id = a.id`, tokenArgs...).
		Order("task_count DESC")

	if orgID != "" {
		query = query.Where("a.org_id = ?", orgID)
	}
	if projectID != "" {
		query = query.Where("a.id IN (SELECT agent_id FROM project_agents WHERE project_id = ?)", projectID)
	}

	var stats []models.AgentStats
	if err := query.Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("agent performance: %w", err)
	}
	return stats, nil
}
