package service

import (
	"context"
	"log/slog"
	"sort"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

const rrfK = 60 // Standard RRF constant

// Search performs a dual-stream search (BM25 + Graph) and merges results via RRF.
//
// There used to be a third stream: pgvector similarity over embeddings
// generated through the LLM API layer. That layer is gone, and with it the
// only embedder — see docs/openspecs/remove-api-execution-mode (D4). BM25 and
// the knowledge graph are pure Postgres and cost nothing to keep, so recall
// degrades from semantic to lexical rather than disappearing.
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

	// Stream 2: Graph search (use top BM25 result as seed if available)
	var graphResults []models.MemorySearchResult
	if len(bm25Results) > 0 {
		graphResults, err = s.memories.SearchGraph(ctx, bm25Results[0].Memory.ID, 2)
		if err != nil {
			slog.Warn("graph search failed, continuing with other streams", "error", err)
		}
	}

	// Merge via RRF
	merged := rrfMerge(bm25Results, graphResults)

	// MMR diversity re-ranking used to run here, dropping near-duplicates by
	// cosine similarity between result embeddings. It went with the vectors:
	// with no embeddings there is nothing to measure similarity over. RRF's
	// own rank fusion still de-emphasizes results that only one stream likes.
	if len(merged) > input.Limit {
		merged = merged[:input.Limit]
	}
	return merged, nil
}

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
