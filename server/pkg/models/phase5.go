package models

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Task 1: Dashboard Analytics Models
// ──────────────────────────────────────────────────────────────────────────────

// OverviewStats provides a high-level platform summary.
type OverviewStats struct {
	TotalProjects    int64   `json:"total_projects"`
	TotalTasks       int64   `json:"total_tasks"`
	ActiveTasks      int64   `json:"active_tasks"`
	CompletedTasks   int64   `json:"completed_tasks"`
	FailedTasks      int64   `json:"failed_tasks"`
	RunningAgents    int64   `json:"running_agents"`
	TotalAgents      int64   `json:"total_agents"`
	SuccessRate      float64 `json:"success_rate"`
	AvgCompletionMs  float64 `json:"avg_completion_ms"`
	OpenPRs          int64   `json:"open_prs"`
	TotalTokenCost   float64 `json:"total_token_cost"`
	TotalTokensUsed  int64   `json:"total_tokens_used"`
}

// AgentStats provides per-agent performance metrics.
type AgentStats struct {
	AgentID      string  `json:"agent_id"`
	AgentName    string  `json:"agent_name"`
	Role         string  `json:"role"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Status       string  `json:"status"`
	TaskCount    int64   `json:"task_count"`
	SuccessCount int64   `json:"success_count"`
	FailCount    int64   `json:"fail_count"`
	SuccessRate  float64 `json:"success_rate"`
	RetryCount   int64   `json:"retry_count"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCostUSD float64 `json:"total_cost_usd"`
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
	Step       string  `json:"step"`
	AvgMs      float64 `json:"avg_ms"`
	TotalRuns  int64   `json:"total_runs"`
	FailCount  int64   `json:"fail_count"`
}

// WorkflowAnalytics is the combined response for workflow analytics.
type WorkflowAnalytics struct {
	TotalWorkflows    int64               `json:"total_workflows"`
	CompletedCount    int64               `json:"completed_count"`
	FailedCount       int64               `json:"failed_count"`
	CompletionRate    float64             `json:"completion_rate"`
	AvgDurationMs     float64             `json:"avg_duration_ms"`
	StepStats         []WorkflowStepStats `json:"step_stats"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Task 2: Audit Log Model
// ──────────────────────────────────────────────────────────────────────────────

// AuditLog represents an immutable record of a critical system action.
type AuditLog struct {
	ID         string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID      *string         `json:"org_id,omitempty" gorm:"type:uuid"`
	UserID     *string         `json:"user_id,omitempty" gorm:"type:uuid"`
	AgentID    *string         `json:"agent_id,omitempty" gorm:"type:uuid"`
	TaskID     *string         `json:"task_id,omitempty" gorm:"type:uuid"`
	Action     string          `json:"action" gorm:"not null"`
	EntityType string          `json:"entity_type" gorm:"not null"`
	EntityID   string          `json:"entity_id" gorm:"default:''"`
	Details    json.RawMessage `json:"details" gorm:"type:jsonb;default:'{}'"`
	IPAddress  string          `json:"ip_address" gorm:"default:''"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogFilter is used to query audit logs with optional filters.
type AuditLogFilter struct {
	OrgID      string `json:"org_id"`
	UserID     string `json:"user_id"`
	AgentID    string `json:"agent_id"`
	TaskID     string `json:"task_id"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	Since      time.Time
	Limit      int
}

// ──────────────────────────────────────────────────────────────────────────────
// Task 3: PR Generation & Review Models
// ──────────────────────────────────────────────────────────────────────────────

// PR risk levels.
const (
	PRRiskLow      = "low"
	PRRiskMedium   = "medium"
	PRRiskHigh     = "high"
	PRRiskCritical = "critical"
)

// PR review statuses.
const (
	PRStatusOpen     = "open"
	PRStatusApproved = "approved"
	PRStatusRejected = "rejected"
	PRStatusMerged   = "merged"
)

// PRSummary represents AI-generated PR information attached to a task.
type PRSummary struct {
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	PRURL        string   `json:"pr_url"`
	ChangedFiles []string `json:"changed_files"`
	RiskLevel    string   `json:"risk_level"`
	RiskReason   string   `json:"risk_reason"`
	Status       string   `json:"status"`
}

// PRRejectInput is the payload for rejecting a PR.
type PRRejectInput struct {
	Feedback string `json:"feedback"`
}

// Audit action constants for structured logging.
const (
	AuditActionTaskCreated       = "task.created"
	AuditActionTaskStatusChanged = "task.status_changed"
	AuditActionTaskExecuted      = "task.executed"
	AuditActionPRCreated         = "pr.created"
	AuditActionPRApproved        = "pr.approved"
	AuditActionPRRejected        = "pr.rejected"
	AuditActionPRMerged          = "pr.merged"
	AuditActionAgentAssigned     = "agent.assigned"
	AuditActionSecretAccessed    = "secret.accessed"
	AuditActionRuleModified      = "rule.modified"
	AuditActionWorkflowStarted   = "workflow.started"
	AuditActionWorkflowCompleted = "workflow.completed"
	AuditActionWorkflowFailed    = "workflow.failed"
)
