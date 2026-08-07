package models

import (
	"encoding/json"
	"time"
)

// TaskEvent is a single entry in a task's append-only agent-activity stream
// (docs/openspecs/status-driven-agent-workspace/design.md). SequenceNumber,
// not CreatedAt, is the ordering/cursor key — see design.md's Ordering
// section for why CreatedAt alone is unsafe under concurrent writers.
type TaskEvent struct {
	ID             string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID         string          `json:"task_id" gorm:"type:uuid;not null"`
	SequenceNumber int64           `json:"sequence_number" gorm:"not null"`
	Type           string          `json:"type" gorm:"not null"`
	SchemaVersion  int             `json:"schema_version" gorm:"not null;default:1"`
	Payload        json.RawMessage `json:"payload" gorm:"type:jsonb;default:'{}'"`
	ArtifactID     *string         `json:"artifact_id,omitempty" gorm:"type:uuid"`
	SizeBytes      int             `json:"size_bytes" gorm:"not null;default:0"`
	CreatedAt      time.Time       `json:"created_at"`
}

// AgentEventType enumerates the well-known TaskEvent.Type values emitted by
// the agent adapter (design.md's Event Types section).
const (
	AgentEventTaskStarted      = "task.started"
	AgentEventTaskCompleted    = "task.completed"
	AgentEventTaskError        = "task.error"
	AgentEventStatusChanged    = "status.changed"
	AgentEventReasoningSummary = "agent.reasoning_summary"
	AgentEventPlan             = "agent.plan"
	AgentEventMessage          = "agent.message"
	AgentEventToolStarted      = "tool.started"
	AgentEventToolFinished     = "tool.finished"
	AgentEventFileChanged      = "file.changed"
	AgentEventCommandStarted   = "command.started"
	AgentEventCommandFinished  = "command.finished"
	AgentEventTestResult       = "test.result"
)

// Payload size caps (design.md's Payload Size Limits section). Payloads
// larger than MaxPayloadBytes must be externalized to workflow_artifacts
// and referenced via TaskEvent.ArtifactID; MaxTailBytes bounds any single
// tail/log-like field retained inline alongside an externalized payload.
const (
	MaxPayloadBytes = 8192
	MaxTailBytes    = 5120
)
