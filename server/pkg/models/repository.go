package models

import "time"

// Repository represents a Git repository linked to a project.
type Repository struct {
	ID              string     `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID       string     `json:"project_id" gorm:"type:uuid;not null"`
	GitAccountID    *string    `json:"git_account_id,omitempty" gorm:"type:uuid;default:null"`
	URL             string     `json:"url" gorm:"not null"`
	Provider        string     `json:"provider" gorm:"default:'github'"`
	Branch          string     `json:"branch" gorm:"default:'main'"`
	Token           string     `json:"-" gorm:"column:encrypted_token;default:''"` // never expose in JSON
	ClonePath       string     `json:"clone_path" gorm:"default:''"`
	CloneStatus     string     `json:"clone_status" gorm:"default:'not_cloned'"`
	LastValidatedAt *time.Time `json:"last_validated_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateRepositoryInput is the payload to create a repository.
type CreateRepositoryInput struct {
	URL          string  `json:"url"`
	Provider     string  `json:"provider"`
	Branch       string  `json:"branch"`
	Token        string  `json:"token"`
	GitAccountID *string `json:"git_account_id"`
}

// UpdateRepositoryInput is the payload to partially update a repository.
type UpdateRepositoryInput struct {
	URL          *string `json:"url,omitempty"`
	Provider     *string `json:"provider,omitempty"`
	Branch       *string `json:"branch,omitempty"`
	Token        *string `json:"token,omitempty"`
	GitAccountID *string `json:"git_account_id,omitempty"`
	ClonePath    *string `json:"clone_path,omitempty"`
	CloneStatus  *string `json:"clone_status,omitempty"`
}

type RemoteRepository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}
