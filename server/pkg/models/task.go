package models

import (
	"time"

	"github.com/lib/pq"
)

// Task statuses — full lifecycle.
const (
	TaskStatusTodo        = "todo"
	TaskStatusAssigned    = "assigned"
	TaskStatusPlanning    = "planning"
	TaskStatusCoding      = "coding"
	TaskStatusReviewing   = "reviewing"
	TaskStatusFixing      = "fixing"
	TaskStatusTesting     = "testing"
	TaskStatusHumanReview = "human_review"
	TaskStatusMerged      = "merged"
)

// Task complexity levels.
const (
	TaskComplexityEasy   = "easy"
	TaskComplexityMedium = "medium"
	TaskComplexityHard   = "hard"
)

// ValidTaskTransitions defines allowed status transitions.
var ValidTaskTransitions = map[string][]string{
	TaskStatusTodo:        {TaskStatusAssigned},
	TaskStatusAssigned:    {TaskStatusPlanning, TaskStatusCoding},
	TaskStatusPlanning:    {TaskStatusCoding, TaskStatusTodo},
	TaskStatusCoding:      {TaskStatusReviewing},
	TaskStatusReviewing:   {TaskStatusFixing, TaskStatusTesting},
	TaskStatusFixing:      {TaskStatusReviewing},
	TaskStatusTesting:     {TaskStatusHumanReview, TaskStatusFixing},
	TaskStatusHumanReview: {TaskStatusMerged, TaskStatusFixing},
	TaskStatusMerged:      {},
}

// Task represents a unit of work for an agent.
type Task struct {
	ID          string         `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID   string         `json:"project_id" gorm:"type:uuid;not null"`
	AgentID     *string        `json:"agent_id,omitempty" gorm:"type:uuid"`
	Title       string         `json:"title" gorm:"not null"`
	Description string         `json:"description" gorm:"default:''"`
	Status      string         `json:"status" gorm:"default:'todo'"`
	Complexity  string         `json:"complexity" gorm:"default:'easy'"`
	Priority    int            `json:"priority" gorm:"default:0"`
	Labels      pq.StringArray `json:"labels" gorm:"type:text[];default:'{}'"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// CreateTaskInput is the payload to create a task.
type CreateTaskInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Complexity  string   `json:"complexity"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
}

// UpdateTaskInput is the payload to partially update a task.
type UpdateTaskInput struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Status      *string  `json:"status,omitempty"`
	Complexity  *string  `json:"complexity,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AgentID     *string  `json:"agent_id,omitempty"`
}
