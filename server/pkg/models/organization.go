package models

import (
	"encoding/json"
	"time"
)

// Organization represents the top-level tenant.
type Organization struct {
	ID          string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description" gorm:"default:''"`
	// DefaultExecutionProviders is the org-wide fallback priority list used
	// when a project has no execution_providers of its own and no explicit
	// legacy execution_engine="cli" — see execution_router.go's precedence
	// chain (docs/openspecs/global-execution-providers). Same shape/validator
	// as Project.ExecutionProviders (models.ValidateExecutionProviders).
	DefaultExecutionProviders json.RawMessage `json:"default_execution_providers,omitempty" gorm:"column:default_execution_providers;type:jsonb;default:'[]'"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
}

// CreateOrganizationInput is the payload to create an organization.
type CreateOrganizationInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateOrganizationInput is the payload to partially update an organization.
type UpdateOrganizationInput struct {
	Name                      *string         `json:"name,omitempty"`
	Description               *string         `json:"description,omitempty"`
	DefaultExecutionProviders json.RawMessage `json:"default_execution_providers,omitempty"`
}
