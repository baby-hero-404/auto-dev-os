package models

import "time"

type TokenUsage struct {
	ID               string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID            *string   `json:"org_id,omitempty" gorm:"type:uuid"`
	CredentialID     *string   `json:"credential_id,omitempty" gorm:"type:uuid"`
	ProjectID        *string   `json:"project_id,omitempty" gorm:"type:uuid"`
	AgentID          *string   `json:"agent_id,omitempty" gorm:"type:uuid"`
	TaskID           *string   `json:"task_id,omitempty" gorm:"type:uuid"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	LevelGroup       string    `json:"level_group"`
	PromptTokens     int       `json:"prompt_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	CacheReadTokens  int       `json:"cache_read_tokens"`
	CacheWriteTokens int       `json:"cache_write_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	LatencyMS        int64     `json:"latency_ms"`
	Status           string    `json:"status"`
	Error            string    `json:"error"`
	CreatedAt        time.Time `json:"created_at"`
}

func (TokenUsage) TableName() string {
	return "token_usage"
}

type TokenUsageSummary struct {
	ProjectID       *string `json:"project_id,omitempty"`
	CredentialID    *string `json:"credential_id,omitempty"`
	KeyLabel        string  `json:"key_label,omitempty"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	LevelGroup      string  `json:"level_group"`
	Requests        int64   `json:"requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	PromptTokens    int64   `json:"prompt_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	CostUSD         float64 `json:"cost_usd"`
	AvgLatencyMS    float64 `json:"avg_latency_ms"`
}

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
	TotalTokenCost  float64 `json:"total_token_cost"`
	TotalTokensUsed int64   `json:"total_tokens_used"`
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
	TotalTokens     int64   `json:"total_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
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

// GatewayUsagePoint represents daily LLM gateway usage.
type GatewayUsagePoint struct {
	Bucket       time.Time `json:"bucket"`
	Requests     int64     `json:"requests"`
	PromptTokens int64     `json:"prompt_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	TotalTokens  int64     `json:"total_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	AvgLatencyMS float64   `json:"avg_latency_ms"`
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
