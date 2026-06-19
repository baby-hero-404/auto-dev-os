package models

import "time"

// GitAccount represents a Git provider account credentials configured at organization level.
type GitAccount struct {
	ID          string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID       string    `json:"org_id" gorm:"type:uuid;not null"`
	Provider    string    `json:"provider" gorm:"not null"` // 'github', 'gitlab', 'bitbucket'
	DisplayName string    `json:"display_name" gorm:"not null"`
	BaseURL     string    `json:"base_url" gorm:"default:''"`
	Token       string    `json:"-" gorm:"column:encrypted_token;default:''"` // never expose in JSON
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateGitAccountInput is the payload to create a git account.
type CreateGitAccountInput struct {
	Provider    string `json:"provider"`
	DisplayName string `json:"display_name"`
	BaseURL     string `json:"base_url"`
	Token       string `json:"token"`
}

// UpdateGitAccountInput is the payload to partially update a git account.
type UpdateGitAccountInput struct {
	DisplayName *string `json:"display_name,omitempty"`
	BaseURL     *string `json:"base_url,omitempty"`
	Token       *string `json:"token,omitempty"`
}
