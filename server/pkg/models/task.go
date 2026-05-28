package models

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// Task statuses — full lifecycle.
const (
	TaskStatusTodo        = "todo"
	TaskStatusAnalyzing   = "analyzing"
	TaskStatusSpecReview  = "spec_review"
	TaskStatusAssigned    = "assigned"
	TaskStatusPlanning    = "planning"
	TaskStatusCoding      = "coding"
	TaskStatusReviewing   = "reviewing"
	TaskStatusFixing      = "fixing"
	TaskStatusTesting     = "testing"
	TaskStatusHumanReview = "human_review"
	TaskStatusMerged      = "merged"
	TaskStatusInProgress  = "in_progress"
	TaskStatusFailed      = "failed"
	TaskStatusCompleted   = "completed"
)

// Task complexity levels.
const (
	TaskComplexityEasy   = "easy"
	TaskComplexityMedium = "medium"
	TaskComplexityHard   = "hard"
)

// ValidTaskTransitions defines allowed status transitions.
var ValidTaskTransitions = map[string][]string{
	TaskStatusTodo:        {TaskStatusAnalyzing, TaskStatusAssigned},
	TaskStatusAnalyzing:   {TaskStatusSpecReview, TaskStatusPlanning, TaskStatusCoding, TaskStatusFailed},
	TaskStatusSpecReview:  {TaskStatusPlanning, TaskStatusCoding, TaskStatusTodo, TaskStatusFailed},
	TaskStatusAssigned:    {TaskStatusPlanning, TaskStatusCoding, TaskStatusInProgress, TaskStatusFailed},
	TaskStatusPlanning:    {TaskStatusCoding, TaskStatusTodo, TaskStatusInProgress, TaskStatusFailed},
	TaskStatusCoding:      {TaskStatusReviewing, TaskStatusFailed},
	TaskStatusReviewing:   {TaskStatusFixing, TaskStatusTesting, TaskStatusFailed},
	TaskStatusFixing:      {TaskStatusReviewing, TaskStatusFailed},
	TaskStatusTesting:     {TaskStatusHumanReview, TaskStatusFixing, TaskStatusFailed},
	TaskStatusHumanReview: {TaskStatusMerged, TaskStatusFixing, TaskStatusFailed},
	TaskStatusMerged:      {},
	TaskStatusInProgress:  {TaskStatusHumanReview, TaskStatusCompleted, TaskStatusFailed},
	TaskStatusFailed:      {TaskStatusTodo, TaskStatusInProgress},
	TaskStatusCompleted:   {},
}

const (
	TaskSpecStatusNone             = "none"
	TaskSpecStatusDraft            = "draft"
	TaskSpecStatusPendingReview    = "pending_review"
	TaskSpecStatusChangesRequested = "changes_requested"
	TaskSpecStatusApproved         = "approved"
	TaskSpecStatusAutoApproved     = "auto_approved"
)

// Task represents a unit of work for an agent.
type Task struct {
	ID           string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID    string          `json:"project_id" gorm:"type:uuid;not null"`
	AgentID      *string         `json:"agent_id,omitempty" gorm:"type:uuid"`
	ParentTaskID *string         `json:"parent_task_id,omitempty" gorm:"type:uuid"`
	Title        string          `json:"title" gorm:"not null"`
	Description  string          `json:"description" gorm:"default:''"`
	Status       string          `json:"status" gorm:"default:'todo'"`
	Complexity   string          `json:"complexity" gorm:"default:'easy'"`
	Priority     int             `json:"priority" gorm:"default:0"`
	Labels       pq.StringArray  `json:"labels" gorm:"type:text[];default:'{}'"`
	Analysis     json.RawMessage `json:"analysis" gorm:"type:jsonb;default:'{}'"`
	SpecStatus   string          `json:"spec_status" gorm:"default:'none'"`
	SubTasks     []Task          `json:"subtasks,omitempty" gorm:"foreignKey:ParentTaskID"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// CreateTaskInput is the payload to create a task.
type CreateTaskInput struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Complexity   string   `json:"complexity"`
	Priority     int      `json:"priority"`
	Labels       []string `json:"labels"`
	ParentTaskID *string  `json:"parent_task_id,omitempty"`
}

// UpdateTaskInput is the payload to partially update a task.
type UpdateTaskInput struct {
	Title        *string         `json:"title,omitempty"`
	Description  *string         `json:"description,omitempty"`
	Status       *string         `json:"status,omitempty"`
	Complexity   *string         `json:"complexity,omitempty"`
	Priority     *int            `json:"priority,omitempty"`
	Labels       []string        `json:"labels,omitempty"`
	AgentID      *string         `json:"agent_id,omitempty"`
	Analysis     json.RawMessage `json:"analysis,omitempty"`
	SpecStatus   *string         `json:"spec_status,omitempty"`
	ParentTaskID *string         `json:"parent_task_id,omitempty"`
}

type TaskAnalysis struct {
	Complexity             string   `json:"complexity"`
	Scope                  string   `json:"scope"`
	AffectedFiles          []string `json:"affected_files"`
	Risks                  []string `json:"risks"`
	ExecutionPlan          []string `json:"execution_plan"`
	ClarificationQuestions []string `json:"clarification_questions,omitempty"`
}

type ClarifyTaskInput struct {
	Context string `json:"context"`
}
