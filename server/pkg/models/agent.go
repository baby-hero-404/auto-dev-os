package models

import "time"

// Agent roles.
const (
	AgentRolePlanner  = "planner"
	AgentRoleBackend  = "backend"
	AgentRoleFrontend = "frontend"
	AgentRoleReviewer = "reviewer"
	AgentRoleQA       = "qa"
)

// Agent levels.
const (
	AgentLevelEasy   = "easy"
	AgentLevelMedium = "medium"
	AgentLevelHard   = "hard"
)

// Agent statuses.
const (
	AgentStatusIdle    = "idle"
	AgentStatusBusy    = "busy"
	AgentStatusOffline = "offline"
)

// Agent represents an AI worker with a role and model configuration.
type Agent struct {
	ID        string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID string    `json:"project_id" gorm:"type:uuid;not null"`
	Name      string    `json:"name" gorm:"not null"`
	Role      string    `json:"role" gorm:"default:'backend'"`
	Provider  string    `json:"provider" gorm:"default:'openai'"`
	Model     string    `json:"model" gorm:"default:'gpt-4o'"`
	Level     string    `json:"level" gorm:"default:'easy'"`
	Status    string    `json:"status" gorm:"default:'idle'"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateAgentInput is the payload to create an agent.
type CreateAgentInput struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Level    string `json:"level"`
}

// UpdateAgentInput is the payload to partially update an agent.
type UpdateAgentInput struct {
	Name     *string `json:"name,omitempty"`
	Role     *string `json:"role,omitempty"`
	Provider *string `json:"provider,omitempty"`
	Model    *string `json:"model,omitempty"`
	Level    *string `json:"level,omitempty"`
	Status   *string `json:"status,omitempty"`
}
