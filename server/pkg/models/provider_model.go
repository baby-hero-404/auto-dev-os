package models

import "time"

const (
	ModelLevelFast     = "fast"
	ModelLevelBalanced = "balanced"
	ModelLevelPowerful = "powerful"
)

type ProviderModel struct {
	ID         string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID      string    `json:"org_id" gorm:"type:uuid;not null"`
	Provider   string    `json:"provider" gorm:"not null"`
	LevelGroup string    `json:"level_group" gorm:"not null"`
	ModelName  string    `json:"model_name" gorm:"not null"`
	Priority   int       `json:"priority" gorm:"default:0;not null"`
	IsActive   bool      `json:"is_active" gorm:"default:true;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateProviderModelInput struct {
	Provider   string `json:"provider"`
	LevelGroup string `json:"level_group"`
	ModelName  string `json:"model_name"`
	Priority   int    `json:"priority"`
	IsActive   *bool  `json:"is_active"`
}

type UpdateProviderModelInput struct {
	Provider   *string `json:"provider,omitempty"`
	LevelGroup *string `json:"level_group,omitempty"`
	ModelName  *string `json:"model_name,omitempty"`
	Priority   *int    `json:"priority,omitempty"`
	IsActive   *bool   `json:"is_active,omitempty"`
}

type ProviderModelFilter struct {
	Provider   *string `json:"provider"`
	LevelGroup *string `json:"level_group"`
}

type ComboEntry struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Priority   int    `json:"priority"`
	LevelGroup string `json:"level_group,omitempty"`
}
