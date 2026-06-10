package models

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Suggestion Type & Status Constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	SuggestionTypeRule        = "rule"
	SuggestionTypePromptPatch = "prompt_patch"
	SuggestionTypeSkill       = "skill"
	SuggestionTypePattern     = "pattern"
)

const (
	SuggestionStatusPending  = "pending"
	SuggestionStatusApproved = "approved"
	SuggestionStatusRejected = "rejected"
	SuggestionStatusApplied  = "applied"
)

// ──────────────────────────────────────────────────────────────────────────────
// LearningSuggestion — HITL queue for agent self-improvement
// ──────────────────────────────────────────────────────────────────────────────

// LearningSuggestion represents a proposed improvement from the learning loop.
type LearningSuggestion struct {
	ID             string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	AgentID        string          `json:"agent_id" gorm:"type:uuid;not null"`
	ProjectID      *string         `json:"project_id,omitempty" gorm:"type:uuid"`
	TaskID         *string         `json:"task_id,omitempty" gorm:"type:uuid"`
	SuggestionType string          `json:"suggestion_type" gorm:"default:'rule'"`
	Title          string          `json:"title" gorm:"not null"`
	Description    string          `json:"description" gorm:"default:''"`
	Content        string          `json:"content" gorm:"default:''"`
	Confidence     float64         `json:"confidence" gorm:"default:0.5"`
	Status         string          `json:"status" gorm:"default:'pending'"`
	ReviewedBy     *string         `json:"reviewed_by,omitempty" gorm:"type:uuid"`
	ReviewedAt     *time.Time      `json:"reviewed_at,omitempty"`
	Metadata       json.RawMessage `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (LearningSuggestion) TableName() string {
	return "learning_suggestions"
}

// CreateSuggestionInput is the payload for proposing a learning suggestion.
type CreateSuggestionInput struct {
	AgentID        string  `json:"agent_id"`
	ProjectID      *string `json:"project_id,omitempty"`
	TaskID         *string `json:"task_id,omitempty"`
	SuggestionType string  `json:"suggestion_type"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Content        string  `json:"content"`
	Confidence     float64 `json:"confidence"`
}

// UpdateSuggestionInput is the payload for approving or rejecting a suggestion.
type UpdateSuggestionInput struct {
	Status     *string `json:"status,omitempty"`
	ReviewedBy *string `json:"reviewed_by,omitempty"`
	Feedback   *string `json:"feedback,omitempty"` // Stored in metadata
}
