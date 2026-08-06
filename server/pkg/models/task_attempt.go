package models

import "time"

// TaskAttempt records one execution attempt of a Task (parent or child).
// A Task answers "what is this work item"; a TaskAttempt answers "what
// happened the last time (or Nth time) we tried it" — start/end time, exit
// status, tokens in/out, cost, and which sandbox/container ran it. Retrying
// a Task creates a new TaskAttempt row; it never mutates or deletes a prior
// one, preserving forensic history (docs/openspecs/task-subtask-decomposition).
type TaskAttempt struct {
	ID             string     `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID         string     `json:"task_id" gorm:"type:uuid;not null"`
	AttemptNumber  int        `json:"attempt_number" gorm:"not null"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	ExitStatus     *int       `json:"exit_status,omitempty"`
	TokensIn       *int       `json:"tokens_in,omitempty"`
	TokensOut      *int       `json:"tokens_out,omitempty"`
	CostUSD        *float64   `json:"cost_usd,omitempty"`
	SandboxRef     string     `json:"sandbox_ref,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName pins the GORM table name explicitly (plural, matching the
// migration) rather than relying on GORM's pluralization inference.
func (TaskAttempt) TableName() string { return "task_attempts" }

// FinalizeTaskAttemptInput is the payload to close out a TaskAttempt once
// its execution has finished (success or failure).
type FinalizeTaskAttemptInput struct {
	ExitStatus *int     `json:"exit_status,omitempty"`
	TokensIn   *int     `json:"tokens_in,omitempty"`
	TokensOut  *int     `json:"tokens_out,omitempty"`
	CostUSD    *float64 `json:"cost_usd,omitempty"`
}
