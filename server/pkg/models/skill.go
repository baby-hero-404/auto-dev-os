package models

import (
	"encoding/json"
	"time"
)

// Skill represents a reusable action an agent can perform.
type Skill struct {
	ID          string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Name        string          `json:"name" gorm:"uniqueIndex;not null"`
	Description string          `json:"description" gorm:"default:''"`
	Schema      json.RawMessage `json:"schema" gorm:"type:jsonb;default:'{}'"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// CreateSkillInput is the payload to create a skill.
type CreateSkillInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// UpdateSkillInput is the payload to partially update a skill.
type UpdateSkillInput struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Schema      *json.RawMessage `json:"schema,omitempty"`
}
