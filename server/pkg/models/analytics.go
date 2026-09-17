package models

import "time"

// OverviewStats provides a high-level platform summary.
type OverviewStats struct {
	TotalProjects   int64   `json:"total_projects"`
	TotalTasks      int64   `json:"total_tasks"`
	ActiveTasks     int64   `json:"active_tasks"`
	CompletedTasks  int64   `json:"completed_tasks"`
	FailedTasks     int64   `json:"failed_tasks"`
	RunningAgents   int64   `json:"running_agents"`
	TotalAgents     int64   `json:"total_agents"`
	SuccessRate     float64 `json:"success_rate"`
	AvgCompletionMs float64 `json:"avg_completion_ms"`
	OpenPRs         int64   `json:"open_prs"`
}

// AgentStats provides per-agent performance metrics.
type AgentStats struct {
	AgentID         string  `json:"agent_id"`
	AgentName       string  `json:"agent_name"`
	Role            string  `json:"role"`
	ModelLevelGroup string  `json:"model_level_group"`
	Status          string  `json:"status"`
	TaskCount       int64   `json:"task_count"`
	SuccessCount    int64   `json:"success_count"`
	FailCount       int64   `json:"fail_count"`
	SuccessRate     float64 `json:"success_rate"`
	RetryCount      int64   `json:"retry_count"`
}

// TaskTimeSeries represents time-bucketed task counts for trend charts.
type TaskTimeSeries struct {
	Bucket    time.Time `json:"bucket"`
	Created   int64     `json:"created"`
	Completed int64     `json:"completed"`
	Failed    int64     `json:"failed"`
}

// TaskStatusDistribution represents count per status.
type TaskStatusDistribution struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// TaskAnalytics is the combined response for the tasks analytics endpoint.
type TaskAnalytics struct {
	Distribution []TaskStatusDistribution `json:"distribution"`
	TimeSeries   []TaskTimeSeries         `json:"time_series"`
}

// WorkflowStepStats represents average duration for a specific workflow step.
type WorkflowStepStats struct {
	Step      string  `json:"step"`
	AvgMs     float64 `json:"avg_ms"`
	TotalRuns int64   `json:"total_runs"`
	FailCount int64   `json:"fail_count"`
}

// WorkflowAnalytics is the combined response for workflow analytics.
type WorkflowAnalytics struct {
	TotalWorkflows int64               `json:"total_workflows"`
	CompletedCount int64               `json:"completed_count"`
	FailedCount    int64               `json:"failed_count"`
	CompletionRate float64             `json:"completion_rate"`
	AvgDurationMs  float64             `json:"avg_duration_ms"`
	StepStats      []WorkflowStepStats `json:"step_stats"`
}

// RecentFailure captures a failed task and the most useful workflow error attached to it.
type RecentFailure struct {
	TaskID        string    `json:"task_id"`
	ProjectID     string    `json:"project_id"`
	ProjectName   string    `json:"project_name"`
	Title         string    `json:"title"`
	FailureReason string    `json:"failure_reason"`
	WorkflowStep  string    `json:"workflow_step"`
	FailedAt      time.Time `json:"failed_at"`
}
