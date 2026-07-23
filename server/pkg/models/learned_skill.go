package models

import (
	"time"

	"github.com/lib/pq"
)

// LearnedSkill statuses.
const (
	LearnedSkillStatusDraft    = "draft"
	LearnedSkillStatusActive   = "active"
	LearnedSkillStatusDisabled = "disabled"
)

// LearnedSkill is a reusable pattern extracted from a merged task, distinct
// from the agent tool/plugin catalog's Skill model — this one is loaded into
// context_load for future tasks whose title/description matches
// TriggerKeywords, not invoked as a callable tool.
type LearnedSkill struct {
	ID              string         `json:"id" gorm:"primaryKey;default:uuid_generate_v4()"`
	ProjectID       string         `json:"project_id" gorm:"column:project_id;not null"`
	Title           string         `json:"title" gorm:"not null"`
	TriggerKeywords pq.StringArray `json:"trigger_keywords" gorm:"type:text[];default:'{}'"`
	Content         string         `json:"content" gorm:"not null"`
	Status          string         `json:"status" gorm:"not null;default:draft"`
	SourceTaskID    *string        `json:"source_task_id" gorm:"column:source_task_id"`
	UsageCount      int            `json:"usage_count" gorm:"not null;default:0"`
	SuccessCount    int            `json:"success_count" gorm:"not null;default:0"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (LearnedSkill) TableName() string { return "learned_skills" }

// CreateLearnedSkillInput is the payload for extracting/creating a learned skill.
type CreateLearnedSkillInput struct {
	ProjectID       string
	Title           string
	TriggerKeywords []string
	Content         string
	Status          string
	SourceTaskID    *string
}

// UpdateLearnedSkillInput is the payload for editing/approving/disabling a learned skill.
type UpdateLearnedSkillInput struct {
	Title           *string  `json:"title,omitempty"`
	TriggerKeywords []string `json:"trigger_keywords,omitempty"`
	Content         *string  `json:"content,omitempty"`
	Status          *string  `json:"status,omitempty"`
}
