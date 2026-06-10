package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// MemoryRepo handles CRUD and search operations for episodic memories.
type MemoryRepo struct{ db *gorm.DB }

func NewMemoryRepo(db *gorm.DB) *MemoryRepo {
	return &MemoryRepo{db: db}
}

// Create inserts a new episodic memory with content-hash deduplication.
func (r *MemoryRepo) Create(ctx context.Context, mem *models.EpisodicMemory) error {
	if mem.ContentHash == "" {
		mem.ContentHash = contentHash(mem.Content)
	}
	// Dedup check: skip if identical hash exists for same agent + session
	var count int64
	q := r.db.WithContext(ctx).Model(&models.EpisodicMemory{}).
		Where("agent_id = ? AND content_hash = ?", mem.AgentID, mem.ContentHash)
	if mem.SessionID != nil {
		q = q.Where("session_id = ?", *mem.SessionID)
	}
	if err := q.Count(&count).Error; err != nil {
		return fmt.Errorf("check memory dedup: %w", err)
	}
	if count > 0 {
		return nil // duplicate, skip silently
	}
	if err := r.db.WithContext(ctx).Create(mem).Error; err != nil {
		return fmt.Errorf("create memory: %w", err)
	}
	return nil
}

// GetByID fetches a single memory and bumps access_count + last_accessed.
func (r *MemoryRepo) GetByID(ctx context.Context, id string) (*models.EpisodicMemory, error) {
	var mem models.EpisodicMemory
	if err := r.db.WithContext(ctx).First(&mem, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get memory: %w", mapError(err))
	}
	// Bump access metrics (fire-and-forget)
	r.db.WithContext(ctx).Model(&mem).Updates(map[string]any{
		"access_count":  gorm.Expr("access_count + 1"),
		"last_accessed": time.Now(),
	})
	mem.AccessCount++
	mem.LastAccessed = time.Now()
	return &mem, nil
}

// ListByAgent returns paginated memories filtered by agent and optional tier.
func (r *MemoryRepo) ListByAgent(ctx context.Context, agentID, tier string, limit, offset int) ([]models.EpisodicMemory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("created_at DESC")
	if tier != "" {
		query = query.Where("tier = ?", tier)
	}
	var memories []models.EpisodicMemory
	if err := query.Limit(limit).Offset(offset).Find(&memories).Error; err != nil {
		return nil, fmt.Errorf("list memories by agent: %w", err)
	}
	return memories, nil
}

// ListBySession returns all memories for a given session.
func (r *MemoryRepo) ListBySession(ctx context.Context, sessionID string) ([]models.EpisodicMemory, error) {
	var memories []models.EpisodicMemory
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at ASC").Find(&memories).Error; err != nil {
		return nil, fmt.Errorf("list memories by session: %w", err)
	}
	return memories, nil
}

// SearchBM25Ranked performs full-text search and returns memories with proper ranking scores.
func (r *MemoryRepo) SearchBM25Ranked(ctx context.Context, query, agentID string, limit int) ([]models.MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	type rankedRow struct {
		models.EpisodicMemory
		BM25Score float64 `gorm:"column:bm25_score"`
	}
	var rows []rankedRow

	if err := r.db.WithContext(ctx).Raw(`
		SELECT em.*, ts_rank(em.tsv, plainto_tsquery('english', ?)) AS bm25_score
		FROM episodic_memories em
		WHERE em.agent_id = ?
		  AND em.tsv @@ plainto_tsquery('english', ?)
		ORDER BY bm25_score DESC
		LIMIT ?
	`, query, agentID, query, limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("bm25 ranked search: %w", err)
	}

	results := make([]models.MemorySearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, models.MemorySearchResult{
			Memory:    row.EpisodicMemory,
			BM25Score: row.BM25Score,
		})
	}
	return results, nil
}

// SearchVector performs cosine similarity search using pgvector.
// The embedding parameter must be a 1536-dimensional float slice serialized as a pgvector literal.
func (r *MemoryRepo) SearchVector(ctx context.Context, embeddingLiteral, agentID string, limit int) ([]models.MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	type rankedRow struct {
		models.EpisodicMemory
		VectorScore float64 `gorm:"column:vector_score"`
	}
	var rows []rankedRow

	if err := r.db.WithContext(ctx).Raw(`
		SELECT em.*, 1 - (em.embedding <=> ?::vector) AS vector_score
		FROM episodic_memories em
		WHERE em.agent_id = ?
		  AND em.embedding IS NOT NULL
		ORDER BY em.embedding <=> ?::vector
		LIMIT ?
	`, embeddingLiteral, agentID, embeddingLiteral, limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	results := make([]models.MemorySearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, models.MemorySearchResult{
			Memory:      row.EpisodicMemory,
			VectorScore: row.VectorScore,
		})
	}
	return results, nil
}

// SearchGraph traverses knowledge_edges from a source memory up to the given depth.
func (r *MemoryRepo) SearchGraph(ctx context.Context, memoryID string, depth int) ([]models.MemorySearchResult, error) {
	if depth <= 0 {
		depth = 2
	}

	type graphRow struct {
		models.EpisodicMemory
		GraphScore float64 `gorm:"column:graph_score"`
	}
	var rows []graphRow

	// Recursive CTE to traverse edges up to `depth` hops
	if err := r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE graph AS (
			SELECT target_id AS memory_id, weight, 1 AS hop
			FROM knowledge_edges
			WHERE source_id = ?
			UNION ALL
			SELECT ke.target_id, ke.weight * g.weight, g.hop + 1
			FROM knowledge_edges ke
			JOIN graph g ON ke.source_id = g.memory_id
			WHERE g.hop < ?
		)
		SELECT em.*, MAX(g.weight) AS graph_score
		FROM graph g
		JOIN episodic_memories em ON em.id = g.memory_id
		GROUP BY em.id
		ORDER BY graph_score DESC
		LIMIT 20
	`, memoryID, depth).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("graph search: %w", err)
	}

	results := make([]models.MemorySearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, models.MemorySearchResult{
			Memory:     row.EpisodicMemory,
			GraphScore: row.GraphScore,
		})
	}
	return results, nil
}

// UpdateDecay applies Ebbinghaus-style decay to memories not accessed recently.
// Multiplies decay_score by 0.95 for all memories not accessed in the last 7 days.
func (r *MemoryRepo) UpdateDecay(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	result := r.db.WithContext(ctx).
		Model(&models.EpisodicMemory{}).
		Where("last_accessed < ? AND decay_score > 0.01", cutoff).
		Update("decay_score", gorm.Expr("decay_score * 0.95"))
	if result.Error != nil {
		return 0, fmt.Errorf("update decay: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// Delete removes a memory by ID.
func (r *MemoryRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.EpisodicMemory{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	return nil
}

// PruneWorkingMemory removes the oldest working-tier memories for an agent, keeping only `keepCount`.
func (r *MemoryRepo) PruneWorkingMemory(ctx context.Context, agentID string, keepCount int) (int64, error) {
	if keepCount <= 0 {
		keepCount = 100
	}
	result := r.db.WithContext(ctx).Exec(`
		DELETE FROM episodic_memories
		WHERE id IN (
			SELECT id FROM episodic_memories
			WHERE agent_id = ? AND tier = 'working'
			ORDER BY created_at DESC
			OFFSET ?
		)
	`, agentID, keepCount)
	if result.Error != nil {
		return 0, fmt.Errorf("prune working memory: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// UpdateTier updates the tier of a memory (for promotion).
func (r *MemoryRepo) UpdateTier(ctx context.Context, id, tier, summary string) error {
	updates := map[string]any{"tier": tier, "updated_at": time.Now()}
	if summary != "" {
		updates["summary"] = summary
	}
	if err := r.db.WithContext(ctx).Model(&models.EpisodicMemory{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update memory tier: %w", err)
	}
	return nil
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}
