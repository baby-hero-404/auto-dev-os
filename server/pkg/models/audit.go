package models

import (
	"encoding/json"
	"time"
)

// Audit action constants for structured logging.
const (
	AuditActionTaskCreated                   = "task.created"
	AuditActionTaskStatusChanged             = "task.status_changed"
	AuditActionTaskExecuted                  = "task.executed"
	AuditActionPRCreated                     = "pr.created"
	AuditActionPRApproved                    = "pr.approved"
	AuditActionPRRejected                    = "pr.rejected"
	AuditActionPRMerged                      = "pr.merged"
	AuditActionAgentAssigned                 = "agent.assigned"
	AuditActionSecretAccessed                = "secret.accessed"
	AuditActionRuleModified                  = "rule.modified"
	AuditActionWorkflowStarted               = "workflow.started"
	AuditActionWorkflowCompleted             = "workflow.completed"
	AuditActionWorkflowFailed                = "workflow.failed"
	AuditActionMemoryCreated                 = "memory.created"
	AuditActionMemoryPromoted                = "memory.promoted"
	AuditActionMemoryDeleted                 = "memory.deleted"
	AuditActionSuggestionCreated             = "suggestion.created"
	AuditActionSuggestionApproved            = "suggestion.approved"
	AuditActionSuggestionRejected            = "suggestion.rejected"
	AuditActionSuggestionApplied             = "suggestion.applied"
	AuditActionProviderCredentialCreated     = "provider_credential.created"
	AuditActionProviderCredentialUpdated     = "provider_credential.updated"
	AuditActionProviderCredentialDeleted     = "provider_credential.deleted"
	AuditActionProviderCredentialTested      = "provider_credential.tested"
	AuditActionProviderCredentialUsed        = "provider_credential.used"
	AuditActionProviderCredentialRateLimited = "provider_credential.rate_limited"
	AuditActionProviderCredentialRecovered   = "provider_credential.recovered"

)

// AuditLog represents an immutable record of a critical system action.
type AuditLog struct {
	ID         string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID      *string         `json:"org_id,omitempty" gorm:"type:uuid"`
	UserID     *string         `json:"user_id,omitempty" gorm:"type:uuid"`
	AgentID    *string         `json:"agent_id,omitempty" gorm:"type:uuid"`
	TaskID     *string         `json:"task_id,omitempty" gorm:"type:uuid"`
	Action     string          `json:"action" gorm:"not null"`
	EntityType string          `json:"entity_type" gorm:"not null"`
	EntityID   string          `json:"entity_id" gorm:"default:''"`
	Details    json.RawMessage `json:"details" gorm:"type:jsonb;default:'{}'"`
	IPAddress  string          `json:"ip_address" gorm:"default:''"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogFilter is used to query audit logs with optional filters.
type AuditLogFilter struct {
	OrgID      string    `json:"org_id"`
	UserID     string    `json:"user_id"`
	AgentID    string    `json:"agent_id"`
	TaskID     string    `json:"task_id"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	Since      time.Time `json:"since"`
	Limit      int       `json:"limit"`
}
