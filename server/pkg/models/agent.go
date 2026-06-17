package models

import (
	"encoding/json"
	"time"
)

// Agent roles.
const (
	AgentRolePlanner         = "planner"
	AgentRoleBackend         = "backend"
	AgentRoleFrontend        = "frontend"
	AgentRoleReviewer        = "reviewer"
	AgentRoleQA              = "qa"
	AgentRoleSecurityAuditor = "security-auditor"
	AgentRoleDBArchitect     = "db-architect"
)

// Agent statuses.
const (
	AgentStatusIdle     = "idle"
	AgentStatusBusy     = "busy"
	AgentStatusAssigned = "assigned"
	AgentStatusRunning  = "running"
	AgentStatusOffline  = "offline"
)

// Agent assignment strategies.
const (
	AgentAssignmentAutoJoin = "auto_join"
	AgentAssignmentManual   = "manual"
)

// Agent autonomy levels.
const (
	AgentAutonomyAutonomous       = "autonomous"
	AgentAutonomySupervised       = "supervised"
	AgentAutonomyApprovalRequired = "approval_required"
)

// Agent represents an AI worker with a role and capability configuration.
type Agent struct {
	ID                 string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID              string          `json:"org_id" gorm:"type:uuid;not null"`
	Name               string          `json:"name" gorm:"not null"`
	Role               string          `json:"role" gorm:"not null"`
	Goal               string          `json:"goal" gorm:"not null"`
	AutonomyLevel      string          `json:"autonomy_level" gorm:"default:'supervised';not null"`
	ContextConfig      json.RawMessage `json:"context_config" gorm:"type:jsonb;default:'{}';not null"`
	ModelRoute         string          `json:"model_route" gorm:"default:'balanced';not null"`
	Status             string          `json:"status" gorm:"default:'idle'"`
	AssignmentStrategy string          `json:"assignment_strategy" gorm:"default:'manual'"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// CreateAgentInput is the payload to create an agent.
type CreateAgentInput struct {
	Name               string          `json:"name"`
	Role               string          `json:"role"`
	Goal               string          `json:"goal"`
	AutonomyLevel      string          `json:"autonomy_level"`
	ContextConfig      json.RawMessage `json:"context_config,omitempty"`
	ModelRoute         string          `json:"model_route"`
	AssignmentStrategy string          `json:"assignment_strategy"`
	AgentID            string          `json:"agent_id,omitempty"`
}

// UpdateAgentInput is the payload to partially update an agent.
type UpdateAgentInput struct {
	Name               *string          `json:"name,omitempty"`
	Role               *string          `json:"role,omitempty"`
	Goal               *string          `json:"goal,omitempty"`
	AutonomyLevel      *string          `json:"autonomy_level,omitempty"`
	ContextConfig      *json.RawMessage `json:"context_config,omitempty"`
	ModelRoute         *string          `json:"model_route,omitempty"`
	Status             *string          `json:"status,omitempty"`
	AssignmentStrategy *string          `json:"assignment_strategy,omitempty"`
}

type RoleTemplate struct {
	ID           string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Role         string          `json:"role" gorm:"uniqueIndex;not null"`
	DefaultGoal  string          `json:"default_goal" gorm:"not null"`
	DefaultTools json.RawMessage `json:"default_tools" gorm:"type:jsonb;default:'[]';not null"`
	CreatedAt    time.Time       `json:"created_at"`
}

type ProjectAgent struct {
	ProjectID string    `json:"project_id" gorm:"type:uuid;primaryKey"`
	AgentID   string    `json:"agent_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentSkill struct {
	AgentID   string    `json:"agent_id" gorm:"type:uuid;primaryKey"`
	SkillID   string    `json:"skill_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}
