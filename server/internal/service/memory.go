package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// MemoryService manages the 4-tier episodic memory system with triple-stream search.
type MemoryService struct {
	memories *repository.MemoryRepo
	edges    *repository.KnowledgeEdgeRepo
	embedder MemoryEmbedder
}

func NewMemoryService(memories *repository.MemoryRepo, edges *repository.KnowledgeEdgeRepo) *MemoryService {
	return &MemoryService{memories: memories, edges: edges}
}

type MemoryEmbedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
	Name() string
}

func (s *MemoryService) SetEmbedder(embedder MemoryEmbedder) {
	s.embedder = embedder
}

// ──────────────────────────────────────────────────────────────────────────────
// Recording & Promotion
// ──────────────────────────────────────────────────────────────────────────────

// RecordObservation records a new working-tier memory with SHA-256 dedup and secret stripping.
func (s *MemoryService) RecordObservation(ctx context.Context, input models.CreateMemoryInput) (*models.EpisodicMemory, error) {
	if input.Content == "" {
		return nil, ErrValidation("memory content is required")
	}
	if input.AgentID == "" {
		return nil, ErrValidation("agent_id is required")
	}

	// Strip potential secrets before storage
	cleaned := stripSecrets(input.Content)
	hash := sha256Hash(cleaned)
	embedding := input.Embedding
	if len(embedding) == 0 && s.embedder != nil {
		generated, err := s.embedder.Embed(ctx, embeddingText(cleaned, input.Summary))
		if err != nil {
			slog.Warn("memory embedding generation failed, storing memory without vector", "error", err)
		} else {
			embedding = generated
		}
	}

	tier := input.Tier
	if tier == "" {
		tier = models.MemoryTierWorking
	}
	category := input.Category
	if category == "" {
		category = models.MemoryCategoryObservation
	}

	mem := &models.EpisodicMemory{
		AgentID:     input.AgentID,
		ProjectID:   input.ProjectID,
		TaskID:      input.TaskID,
		SessionID:   input.SessionID,
		Tier:        tier,
		Content:     cleaned,
		Summary:     input.Summary,
		ContentHash: hash,
		Category:    category,
		Tags:        input.Tags,
		Embedding:   models.Embedding(embedding),
	}

	if err := s.memories.Create(ctx, mem); err != nil {
		return nil, err
	}
	return mem, nil
}

// PromoteToEpisodic compresses a working memory's content into a summary and promotes it.
func (s *MemoryService) PromoteToEpisodic(ctx context.Context, memoryID string) error {
	mem, err := s.memories.GetByID(ctx, memoryID)
	if err != nil {
		return err
	}
	if mem.Tier != models.MemoryTierWorking {
		return ErrValidation("can only promote working-tier memories to episodic")
	}

	// Auto-generate summary if empty (truncate content for now; full LLM summarization in future)
	summary := mem.Summary
	if summary == "" {
		summary = truncateSummary(mem.Content, 200)
	}
	return s.memories.UpdateTier(ctx, memoryID, models.MemoryTierEpisodic, summary)
}

// PromoteToSemantic generalizes an episodic memory into a reusable fact.
func (s *MemoryService) PromoteToSemantic(ctx context.Context, memoryID string) error {
	mem, err := s.memories.GetByID(ctx, memoryID)
	if err != nil {
		return err
	}
	if mem.Tier != models.MemoryTierEpisodic {
		return ErrValidation("can only promote episodic-tier memories to semantic")
	}
	return s.memories.UpdateTier(ctx, memoryID, models.MemoryTierSemantic, mem.Summary)
}

// PromoteToProcedural marks a memory as a proven procedural pattern.
func (s *MemoryService) PromoteToProcedural(ctx context.Context, memoryID string) error {
	mem, err := s.memories.GetByID(ctx, memoryID)
	if err != nil {
		return err
	}
	if mem.Tier != models.MemoryTierSemantic && mem.Tier != models.MemoryTierEpisodic {
		return ErrValidation("can only promote episodic/semantic memories to procedural")
	}
	return s.memories.UpdateTier(ctx, memoryID, models.MemoryTierProcedural, mem.Summary)
}

// ──────────────────────────────────────────────────────────────────────────────
// Search — Triple-Stream with Reciprocal Rank Fusion (RRF)
// ──────────────────────────────────────────────────────────────────────────────

const rrfK = 60 // Standard RRF constant

// Search performs a triple-stream search (BM25 + Vector + Graph) and merges results via RRF.
func (s *MemoryService) Search(ctx context.Context, input models.MemorySearchInput) ([]models.MemorySearchResult, error) {
	if input.Limit <= 0 {
		input.Limit = 10
	}
	fetchLimit := input.Limit * 3 // Over-fetch for RRF merging

	// Stream 1: BM25 full-text search
	bm25Results, err := s.memories.SearchBM25Ranked(ctx, input.Query, input.AgentID, fetchLimit)
	if err != nil {
		slog.Warn("bm25 search failed, continuing with other streams", "error", err)
	}

	// Stream 2: Vector search. Generate the query embedding when the caller did not provide one.
	var vectorResults []models.MemorySearchResult
	if len(input.Embedding) == 0 && input.Query != "" && s.embedder != nil {
		generated, embedErr := s.embedder.Embed(ctx, input.Query)
		if embedErr != nil {
			slog.Warn("memory query embedding generation failed, continuing without vector stream", "error", embedErr)
		} else {
			input.Embedding = generated
		}
	}
	if len(input.Embedding) > 0 {
		literal := embeddingToLiteral(input.Embedding)
		vectorResults, err = s.memories.SearchVector(ctx, literal, input.AgentID, fetchLimit)
		if err != nil {
			slog.Warn("vector search failed, continuing with other streams", "error", err)
		}
	}

	// Stream 3: Graph search (use top BM25 result as seed if available)
	var graphResults []models.MemorySearchResult
	if len(bm25Results) > 0 {
		graphResults, err = s.memories.SearchGraph(ctx, bm25Results[0].Memory.ID, 2)
		if err != nil {
			slog.Warn("graph search failed, continuing with other streams", "error", err)
		}
	}

	// Merge via RRF
	merged := rrfMerge(bm25Results, vectorResults, graphResults)

	// Apply limit
	if len(merged) > input.Limit {
		merged = merged[:input.Limit]
	}
	return merged, nil
}

func embeddingText(content, summary string) string {
	if strings.TrimSpace(summary) == "" {
		return content
	}
	return summary + "\n\n" + content
}

// ──────────────────────────────────────────────────────────────────────────────
// Maintenance
// ──────────────────────────────────────────────────────────────────────────────

// ApplyDecay runs Ebbinghaus decay on all memories not accessed recently.
func (s *MemoryService) ApplyDecay(ctx context.Context) error {
	affected, err := s.memories.UpdateDecay(ctx)
	if err != nil {
		return err
	}
	if affected > 0 {
		slog.Info("applied memory decay", "affected", affected)
	}
	return nil
}

// CleanupSession prunes working memories from a session and promotes key ones to episodic.
func (s *MemoryService) CleanupSession(ctx context.Context, agentID, sessionID string) error {
	memories, err := s.memories.ListBySession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Promote error and success memories to episodic
	for i := range memories {
		if memories[i].Category == models.MemoryCategoryError ||
			memories[i].Category == models.MemoryCategorySuccess ||
			memories[i].Category == models.MemoryCategoryDecision {
			if err := s.PromoteToEpisodic(ctx, memories[i].ID); err != nil {
				slog.Warn("failed to promote memory", "id", memories[i].ID, "error", err)
			}
		}
	}

	// Prune excess working memories
	if _, err := s.memories.PruneWorkingMemory(ctx, agentID, 100); err != nil {
		slog.Warn("failed to prune working memory", "error", err)
	}
	return nil
}

// ListByAgent returns paginated memories for an agent.
func (s *MemoryService) ListByAgent(ctx context.Context, agentID, tier string, limit, offset int) ([]models.EpisodicMemory, error) {
	return s.memories.ListByAgent(ctx, agentID, tier, limit, offset)
}

// GetByID returns a single memory by ID.
func (s *MemoryService) GetByID(ctx context.Context, id string) (*models.EpisodicMemory, error) {
	return s.memories.GetByID(ctx, id)
}

// Delete removes a memory by ID.
func (s *MemoryService) Delete(ctx context.Context, id string) error {
	return s.memories.Delete(ctx, id)
}

// GetEdgesByMemory returns all outbound + inbound edges for a memory.
func (s *MemoryService) GetEdgesByMemory(ctx context.Context, memoryID string) ([]models.KnowledgeEdge, error) {
	outbound, err := s.edges.ListBySource(ctx, memoryID)
	if err != nil {
		return nil, err
	}
	inbound, err := s.edges.ListByTarget(ctx, memoryID)
	if err != nil {
		return nil, err
	}
	return append(outbound, inbound...), nil
}

// CreateEdge creates a knowledge graph edge between two memories.
func (s *MemoryService) CreateEdge(ctx context.Context, edge *models.KnowledgeEdge) error {
	return s.edges.Create(ctx, edge)
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal Helpers
// ──────────────────────────────────────────────────────────────────────────────

// rrfMerge merges results from multiple search streams using Reciprocal Rank Fusion.
func rrfMerge(streams ...[]models.MemorySearchResult) []models.MemorySearchResult {
	scores := make(map[string]*models.MemorySearchResult) // keyed by memory ID

	for streamIdx, stream := range streams {
		for rank, result := range stream {
			rrfScore := 1.0 / float64(rrfK+rank+1)

			existing, ok := scores[result.Memory.ID]
			if !ok {
				entry := result
				entry.FinalScore = 0
				scores[result.Memory.ID] = &entry
				existing = &entry
			}

			existing.FinalScore += rrfScore

			// Preserve per-stream scores
			switch streamIdx {
			case 0:
				existing.BM25Score = result.BM25Score
			case 1:
				existing.VectorScore = result.VectorScore
			case 2:
				existing.GraphScore = result.GraphScore
			}
		}
	}

	merged := make([]models.MemorySearchResult, 0, len(scores))
	for _, v := range scores {
		merged = append(merged, *v)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].FinalScore > merged[j].FinalScore
	})
	return merged
}

// stripSecrets removes potential API keys, tokens, and passwords from content.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|auth)\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`glpat-[A-Za-z0-9\-]{20,}`),
}

func stripSecrets(content string) string {
	result := content
	for _, re := range secretPatterns {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func truncateSummary(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	// Truncate at word boundary
	truncated := content[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

// embeddingToLiteral converts a float32 slice to a pgvector literal string like '[0.1,0.2,...]'.
func embeddingToLiteral(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
