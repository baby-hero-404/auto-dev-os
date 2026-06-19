package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
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
	Tags        pq.StringArray  `json:"tags" gorm:"type:text[];default:'{}'"`
	Metadata    json.RawMessage `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	Embedding   Embedding       `json:"embedding,omitempty" gorm:"type:vector(1536)"`

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
// Input & Output Structs
// ──────────────────────────────────────────────────────────────────────────────

// CreateMemoryInput is the payload to record a new memory.
type CreateMemoryInput struct {
	AgentID   string    `json:"agent_id"`
	ProjectID *string   `json:"project_id,omitempty"`
	TaskID    *string   `json:"task_id,omitempty"`
	SessionID *string   `json:"session_id,omitempty"`
	Tier      string    `json:"tier"`
	Content   string    `json:"content"`
	Summary   string    `json:"summary"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Embedding []float32 `json:"embedding,omitempty"`
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

// Embedding stores a pgvector-compatible float32 vector.
type Embedding []float32

func (e Embedding) Value() (driver.Value, error) {
	if len(e) == 0 {
		return nil, nil
	}
	parts := make([]string, len(e))
	for i, v := range e {
		parts[i] = strconv.FormatFloat(float64(v), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

func (e *Embedding) Scan(value any) error {
	if value == nil {
		*e = nil
		return nil
	}
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("scan embedding: unsupported type %T", value)
	}
	raw = strings.Trim(raw, "[]")
	if raw == "" {
		*e = nil
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]float32, 0, len(parts))
	for _, part := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return fmt.Errorf("scan embedding: %w", err)
		}
		out = append(out, float32(f))
	}
	*e = out
	return nil
}

// MemorySearchResult is a single result from the triple-stream search with merged scores.
type MemorySearchResult struct {
	Memory      EpisodicMemory `json:"memory"`
	BM25Score   float64        `json:"bm25_score"`
	VectorScore float64        `json:"vector_score"`
	GraphScore  float64        `json:"graph_score"`
	FinalScore  float64        `json:"final_score"` // RRF merged
}
