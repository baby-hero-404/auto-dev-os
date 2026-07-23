package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type MemoryEmbedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
	Name() string
}

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
		generated, embedErr := s.embed(ctx, input.Query)
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

	// Diversify near-duplicates via MMR over the top-2N candidates, then apply limit.
	candidateCount := input.Limit * 2
	if candidateCount > len(merged) {
		candidateCount = len(merged)
	}
	merged = mmrSelect(merged[:candidateCount], input.Limit, mmrLambda)
	return merged, nil
}

const mmrLambda = 0.7

// mmrDuplicateThreshold is the cosine-similarity threshold above which two
// results are considered near-duplicates for diversity purposes (REQ-004).
const mmrDuplicateThreshold = 0.95

// mmrSelect re-ranks candidates (already sorted by relevance, most relevant
// first) using Maximal Marginal Relevance so that near-duplicate results
// (cosine similarity > mmrDuplicateThreshold) don't both make the final N,
// unless there aren't enough distinct candidates to fill it. The first
// (most relevant) result's rank never changes.
func mmrSelect(candidates []models.MemorySearchResult, n int, lambda float64) []models.MemorySearchResult {
	if n <= 0 || len(candidates) == 0 {
		return nil
	}
	if len(candidates) <= n {
		return candidates
	}

	selected := make([]models.MemorySearchResult, 0, n)
	selected = append(selected, candidates[0])
	remaining := candidates[1:]

	for len(selected) < n && len(remaining) > 0 {
		bestIdx := -1
		bestScore := 0.0
		for i, cand := range remaining {
			maxSim := 0.0
			for _, sel := range selected {
				sim := cosineSimilarity(cand.Memory.Embedding, sel.Memory.Embedding)
				if sim > maxSim {
					maxSim = sim
				}
			}
			mmrScore := lambda*cand.FinalScore - (1-lambda)*maxSim
			if bestIdx == -1 || mmrScore > bestScore {
				bestIdx = i
				bestScore = mmrScore
			}
		}
		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	return selected
}

// cosineSimilarity returns the cosine similarity of two embedding vectors.
// A missing/empty or mismatched-length vector (e.g. a memory awaiting
// embedding backfill while the circuit breaker is open) is treated as
// similarity 0 — never a forced duplicate.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func embeddingText(content, summary string) string {
	if strings.TrimSpace(summary) == "" {
		return content
	}
	return summary + "\n\n" + content
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

// embeddingToLiteral converts a float32 slice to a pgvector literal string like '[0.1,0.2,...]'.
func embeddingToLiteral(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
