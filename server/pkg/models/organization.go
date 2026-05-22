package models

import "time"

// Organization represents the top-level tenant.
type Organization struct {
	ID          string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description" gorm:"default:''"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateOrganizationInput is the payload to create an organization.
type CreateOrganizationInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateOrganizationInput is the payload to partially update an organization.
type UpdateOrganizationInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}
