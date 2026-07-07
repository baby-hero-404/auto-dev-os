package models

import "time"

// Rule scopes.
const (
	RuleScopeGlobal  = "global"
	RuleScopeProject = "project"
)

// Rule enforcement levels.
const (
	RuleEnforcementStrict   = "strict"
	RuleEnforcementAdvisory = "advisory"
)

// Rule represents a behavioral constraint for agents.
type Rule struct {
	ID          string         `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID       *string        `json:"org_id,omitempty" gorm:"type:uuid"`
	ProjectID   *string        `json:"project_id,omitempty" gorm:"type:uuid"`
	Scope       string         `json:"scope" gorm:"default:'project'"`
	Content     string         `json:"content" gorm:"not null"`
	Enforcement string         `json:"enforcement" gorm:"default:'strict'"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Roles       []string       `json:"roles,omitempty" gorm:"-"`
	Metadata    map[string]any `json:"metadata,omitempty" gorm:"-"`
}

// CreateRuleInput is the payload to create a rule.
type CreateRuleInput struct {
	Scope       string `json:"scope"`
	Content     string `json:"content"`
	Enforcement string `json:"enforcement"`
}

// UpdateRuleInput is the payload to partially update a rule.
type UpdateRuleInput struct {
	Content     *string `json:"content,omitempty"`
	Enforcement *string `json:"enforcement,omitempty"`
}
