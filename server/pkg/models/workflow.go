package models

import (
	"encoding/json"
	"time"
)

const (
	WorkflowJobStatusQueued  = "queued"
	WorkflowJobStatusRunning = "running"
	WorkflowJobStatusPaused  = "paused"
	WorkflowJobStatusDone    = "done"
	WorkflowJobStatusFailed  = "failed"
)

const (
	WorkflowStepAnalyze      = "analyze"
	WorkflowStepAssign       = "assign"
	WorkflowStepPlan         = "plan"
	WorkflowStepCodeBackend  = "code_backend"
	WorkflowStepCodeFrontend = "code_frontend"
	WorkflowStepMerge        = "merge"
	WorkflowStepReview       = "review"
	WorkflowStepFix          = "fix"
	WorkflowStepTest         = "test"
	WorkflowStepPR           = "pr"
	WorkflowStepSandbox      = "sandbox"
	WorkflowStepDone         = "done"
)

type WorkflowJob struct {
	ID        string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID    string    `json:"task_id" gorm:"type:uuid;not null"`
	AgentID   *string   `json:"agent_id,omitempty" gorm:"type:uuid"`
	Status    string    `json:"status" gorm:"default:'queued'"`
	Step      string    `json:"step" gorm:"default:'analyze'"`
	Attempts  int       `json:"attempts" gorm:"default:0"`
	LastError string    `json:"last_error" gorm:"default:''"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WorkflowCheckpoint struct {
	ID         string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID     string          `json:"task_id" gorm:"type:uuid;not null"`
	JobID      *string         `json:"job_id,omitempty" gorm:"type:uuid"`
	Step       string          `json:"step" gorm:"not null"`
	State      json.RawMessage `json:"state" gorm:"type:jsonb;default:'{}'"`
	CreatedAt  time.Time       `json:"created_at"`
	CommitHash string          `json:"commit_hash,omitempty" gorm:"-"`
}

func (c *WorkflowCheckpoint) GetCommitHash() string {
	if len(c.State) == 0 {
		return ""
	}
	var state map[string]any
	if err := json.Unmarshal(c.State, &state); err != nil {
		return ""
	}
	if h, ok := state["commit_hash"].(string); ok {
		return h
	}
	return ""
}

type TaskLog struct {
	ID        string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID    string    `json:"task_id" gorm:"type:uuid;not null"`
	JobID     *string   `json:"job_id,omitempty" gorm:"type:uuid"`
	Level     string    `json:"level" gorm:"default:'info'"`
	Message   string    `json:"message" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkflowStatus struct {
	Task        *Task                `json:"task"`
	Job         *WorkflowJob         `json:"job,omitempty"`
	Checkpoints []WorkflowCheckpoint `json:"checkpoints"`
}

type WorkflowArtifact struct {
	ID        string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	JobID     string          `json:"job_id" gorm:"type:uuid;not null"`
	TaskID    string          `json:"task_id" gorm:"type:uuid;not null"`
	Step      string          `json:"step" gorm:"not null"`
	Type      string          `json:"type" gorm:"not null"` // e.g. prompt, llm_response, patch, diff, test_output, review_findings
	Payload   json.RawMessage `json:"payload" gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time       `json:"created_at"`
}
