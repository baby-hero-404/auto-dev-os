package models

import (
	"encoding/json"
	"time"
)

// TaskActionRequest deduplicates POST /tasks/{taskID}/actions calls by
// (task_id, request_id) so a double-submit (network retry, double-click)
// re-executes nothing and returns the original response
// (docs/openspecs/status-driven-agent-workspace/design.md's Authorization
// section).
type TaskActionRequest struct {
	ID        string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID    string          `json:"task_id" gorm:"type:uuid;not null"`
	RequestID string          `json:"request_id" gorm:"not null"`
	Action    string          `json:"action" gorm:"not null"`
	Response  json.RawMessage `json:"response" gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time       `json:"created_at"`
}
