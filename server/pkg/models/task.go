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
	TaskStatusCoding      = "coding"
	TaskStatusReviewing   = "reviewing"
	TaskStatusFixing      = "fixing"
	TaskStatusTesting     = "testing"
	TaskStatusHumanReview = "human_review"
	TaskStatusMerged      = "merged"
	TaskStatusFailed      = "failed"
)

// Task complexity levels.
const (
	TaskComplexityEasy   = "easy"
	TaskComplexityMedium = "medium"
	TaskComplexityHard   = "hard"
)

// ValidTaskTransitions defines allowed status transitions.
var ValidTaskTransitions = map[string][]string{
	TaskStatusTodo:        {TaskStatusAnalyzing, TaskStatusCoding},
	TaskStatusAnalyzing:   {TaskStatusSpecReview, TaskStatusCoding, TaskStatusTesting, TaskStatusFailed},
	TaskStatusSpecReview:  {TaskStatusCoding, TaskStatusTodo, TaskStatusFailed},
	TaskStatusCoding:      {TaskStatusReviewing, TaskStatusFailed},
	TaskStatusReviewing:   {TaskStatusFixing, TaskStatusTesting, TaskStatusFailed},
	TaskStatusFixing:      {TaskStatusReviewing, TaskStatusFailed},
	TaskStatusTesting:     {TaskStatusHumanReview, TaskStatusFixing, TaskStatusFailed},
	TaskStatusHumanReview: {TaskStatusMerged, TaskStatusFixing, TaskStatusFailed},
	TaskStatusMerged:      {},
	TaskStatusFailed:      {TaskStatusTodo, TaskStatusAnalyzing},
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
	RepositoryID *string         `json:"repository_id,omitempty" gorm:"type:uuid"`
	Title        string          `json:"title" gorm:"not null"`
	Description  string          `json:"description" gorm:"default:''"`
	Status       string          `json:"status" gorm:"default:'todo'"`
	Complexity   string          `json:"complexity" gorm:"default:'easy'"`
	Priority     int             `json:"priority" gorm:"default:0"`
	Labels       pq.StringArray  `json:"labels" gorm:"type:text[];default:'{}'"`
	Analysis     json.RawMessage `json:"analysis" gorm:"type:jsonb;default:'{}'"`
	SpecStatus   string          `json:"spec_status" gorm:"default:'none'"`
	PRURLs       pq.StringArray  `json:"pr_urls" gorm:"type:text[]"`
	PRMetadata   json.RawMessage `json:"pr_metadata" gorm:"type:jsonb;default:'[]'"`
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
	AgentID      *string  `json:"agent_id,omitempty"`
	RepositoryID *string  `json:"repository_id,omitempty"`
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
	RepositoryID *string         `json:"repository_id,omitempty"`
	Analysis     json.RawMessage `json:"analysis,omitempty"`
	SpecStatus   *string         `json:"spec_status,omitempty"`
	PRURLs       *pq.StringArray `json:"pr_urls,omitempty"`
	PRMetadata   json.RawMessage `json:"pr_metadata,omitempty"`
	ParentTaskID *string         `json:"parent_task_id,omitempty"`
}

type TaskAnalysis struct {
	Complexity             string   `json:"complexity"`
	Scope                  string   `json:"scope"`
	AffectedFiles          []string `json:"affected_files"`
	Risks                  []string `json:"risks"`
	ExecutionPlan          []string `json:"execution_plan"`
	ClarificationQuestions []string `json:"clarification_questions,omitempty"`
	TaskRules              []string `json:"task_rules,omitempty"`
	ProposalMD             string   `json:"proposal_md,omitempty"`
	SpecsMD                string   `json:"specs_md,omitempty"`
	DesignMD               string   `json:"design_md,omitempty"`
	TasksMD                string   `json:"tasks_md,omitempty"`
}

type ClarifyTaskInput struct {
	Context string `json:"context"`
}
