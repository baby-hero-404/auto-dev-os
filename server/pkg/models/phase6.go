package models

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Memory Tier Constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	MemoryTierWorking    = "working"
	MemoryTierEpisodic   = "episodic"
	MemoryTierSemantic   = "semantic"
	MemoryTierProcedural = "procedural"
)

// ──────────────────────────────────────────────────────────────────────────────
// Memory Category Constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	MemoryCategoryObservation  = "observation"
	MemoryCategoryDecision     = "decision"
	MemoryCategoryError        = "error"
	MemoryCategorySuccess      = "success"
	MemoryCategoryPattern      = "pattern"
	MemoryCategoryRule         = "rule"
	MemoryCategoryToolSequence = "tool_sequence"
)

// ──────────────────────────────────────────────────────────────────────────────
// Suggestion Type & Status Constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	SuggestionTypeRule        = "rule"
	SuggestionTypePromptPatch = "prompt_patch"
	SuggestionTypeSkill       = "skill"
	SuggestionTypePattern     = "pattern"
)

const (
	SuggestionStatusPending  = "pending"
	SuggestionStatusApproved = "approved"
	SuggestionStatusRejected = "rejected"
	SuggestionStatusApplied  = "applied"
)

// Knowledge graph relation types.
const (
	EdgeRelationCausedBy   = "caused_by"
	EdgeRelationSolvedBy   = "solved_by"
	EdgeRelationRelatedTo  = "related_to"
	EdgeRelationFollowedBy = "followed_by"
)

// ──────────────────────────────────────────────────────────────────────────────
// EpisodicMemory — 4-tier memory store
// ──────────────────────────────────────────────────────────────────────────────

// EpisodicMemory represents a single memory entry in the 4-tier memory system.
type EpisodicMemory struct {
	ID          string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	AgentID     string          `json:"agent_id" gorm:"type:uuid;not null"`
	ProjectID   *string         `json:"project_id,omitempty" gorm:"type:uuid"`
	TaskID      *string         `json:"task_id,omitempty" gorm:"type:uuid"`
	SessionID   *string         `json:"session_id,omitempty" gorm:"type:uuid"`
	Tier        string          `json:"tier" gorm:"default:'working'"`
	Content     string          `json:"content" gorm:"not null"`
	Summary     string          `json:"summary" gorm:"default:''"`
	ContentHash string          `json:"content_hash" gorm:"default:''"`
	Category    string          `json:"category" gorm:"default:'observation'"`
	Tags        []string        `json:"tags" gorm:"type:text[];default:'{}'"`
	Metadata    json.RawMessage `json:"metadata" gorm:"type:jsonb;default:'{}'"`

	// Decay & reinforcement
	AccessCount  int       `json:"access_count" gorm:"default:0"`
	DecayScore   float64   `json:"decay_score" gorm:"default:1.0"`
	LastAccessed time.Time `json:"last_accessed"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (EpisodicMemory) TableName() string {
	return "episodic_memories"
}

// ──────────────────────────────────────────────────────────────────────────────
// KnowledgeEdge — directed graph relation between memories
// ──────────────────────────────────────────────────────────────────────────────

// KnowledgeEdge represents a directed relation between two episodic memories.
type KnowledgeEdge struct {
	ID        string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	SourceID  string    `json:"source_id" gorm:"type:uuid;not null"`
	TargetID  string    `json:"target_id" gorm:"type:uuid;not null"`
	Relation  string    `json:"relation" gorm:"not null"`
	Weight    float64   `json:"weight" gorm:"default:1.0"`
	CreatedAt time.Time `json:"created_at"`
}

func (KnowledgeEdge) TableName() string {
	return "knowledge_edges"
}

// ──────────────────────────────────────────────────────────────────────────────
// LearningSuggestion — HITL queue for agent self-improvement
// ──────────────────────────────────────────────────────────────────────────────

// LearningSuggestion represents a proposed improvement from the learning loop.
type LearningSuggestion struct {
	ID             string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	AgentID        string          `json:"agent_id" gorm:"type:uuid;not null"`
	ProjectID      *string         `json:"project_id,omitempty" gorm:"type:uuid"`
	TaskID         *string         `json:"task_id,omitempty" gorm:"type:uuid"`
	SuggestionType string          `json:"suggestion_type" gorm:"default:'rule'"`
	Title          string          `json:"title" gorm:"not null"`
	Description    string          `json:"description" gorm:"default:''"`
	Content        string          `json:"content" gorm:"default:''"`
	Confidence     float64         `json:"confidence" gorm:"default:0.5"`
	Status         string          `json:"status" gorm:"default:'pending'"`
	ReviewedBy     *string         `json:"reviewed_by,omitempty" gorm:"type:uuid"`
	ReviewedAt     *time.Time      `json:"reviewed_at,omitempty"`
	Metadata       json.RawMessage `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (LearningSuggestion) TableName() string {
	return "learning_suggestions"
}

// ──────────────────────────────────────────────────────────────────────────────
// Input & Output Structs
// ──────────────────────────────────────────────────────────────────────────────

// CreateMemoryInput is the payload to record a new memory.
type CreateMemoryInput struct {
	AgentID   string   `json:"agent_id"`
	ProjectID *string  `json:"project_id,omitempty"`
	TaskID    *string  `json:"task_id,omitempty"`
	SessionID *string  `json:"session_id,omitempty"`
	Tier      string   `json:"tier"`
	Content   string   `json:"content"`
	Summary   string   `json:"summary"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
}

// MemorySearchInput is the input for triple-stream memory search.
type MemorySearchInput struct {
	Query     string    `json:"query"`
	AgentID   string    `json:"agent_id"`
	ProjectID *string   `json:"project_id,omitempty"`
	Tier      string    `json:"tier,omitempty"`
	Embedding []float32 `json:"embedding,omitempty"`
	Limit     int       `json:"limit"`
}

// MemorySearchResult is a single result from the triple-stream search with merged scores.
type MemorySearchResult struct {
	Memory     EpisodicMemory `json:"memory"`
	BM25Score  float64        `json:"bm25_score"`
	VectorScore float64       `json:"vector_score"`
	GraphScore float64        `json:"graph_score"`
	FinalScore float64        `json:"final_score"` // RRF merged
}

// CreateSuggestionInput is the payload for proposing a learning suggestion.
type CreateSuggestionInput struct {
	AgentID        string  `json:"agent_id"`
	ProjectID      *string `json:"project_id,omitempty"`
	TaskID         *string `json:"task_id,omitempty"`
	SuggestionType string  `json:"suggestion_type"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Content        string  `json:"content"`
	Confidence     float64 `json:"confidence"`
}

// UpdateSuggestionInput is the payload for approving or rejecting a suggestion.
type UpdateSuggestionInput struct {
	Status     *string `json:"status,omitempty"`
	ReviewedBy *string `json:"reviewed_by,omitempty"`
	Feedback   *string `json:"feedback,omitempty"` // Stored in metadata
}

// Audit action constants for Phase 6.
const (
	AuditActionMemoryCreated       = "memory.created"
	AuditActionMemoryPromoted      = "memory.promoted"
	AuditActionMemoryDeleted       = "memory.deleted"
	AuditActionSuggestionCreated   = "suggestion.created"
	AuditActionSuggestionApproved  = "suggestion.approved"
	AuditActionSuggestionRejected  = "suggestion.rejected"
	AuditActionSuggestionApplied   = "suggestion.applied"
)
