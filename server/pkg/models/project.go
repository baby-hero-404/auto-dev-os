package models

import "time"

// Project groups repositories, agents, and rules under an organization.
type Project struct {
	ID          string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID       string    `json:"org_id" gorm:"type:uuid;not null"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description" gorm:"default:''"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateProjectInput is the payload to create a project.
type CreateProjectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateProjectInput is the payload to partially update a project.
type UpdateProjectInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}
