package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestStripSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no secrets",
			input:    "Hello world, this is a clean log.",
			expected: "Hello world, this is a clean log.",
		},
		{
			name:     "api_key parameter",
			input:    "api_key: sk-1234567890abcdef1234567890abcdef",
			expected: "[REDACTED]",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer 1234567890abcdef",
			expected: "Authorization: [REDACTED]",
		},
		{
			name:     "OpenAI key style",
			input:    "Using key sk-1234567890abcdef1234567890abcdef for OpenAI.",
			expected: "Using key [REDACTED] for OpenAI.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripSecrets(tc.input)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestTruncateSummary(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{
			name:     "short content no truncation",
			content:  "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "truncation at space boundary",
			content:  "hello world python golang rust typescript",
			maxLen:   15,
			expected: "hello world...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateSummary(tc.content, tc.maxLen)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRRFMerge(t *testing.T) {
	bm25 := []models.MemorySearchResult{
		{Memory: models.EpisodicMemory{ID: "m1", Summary: "memory 1"}, BM25Score: 10.0},
		{Memory: models.EpisodicMemory{ID: "m2", Summary: "memory 2"}, BM25Score: 8.0},
	}
	vector := []models.MemorySearchResult{
		{Memory: models.EpisodicMemory{ID: "m2", Summary: "memory 2"}, VectorScore: 0.9},
		{Memory: models.EpisodicMemory{ID: "m3", Summary: "memory 3"}, VectorScore: 0.8},
	}
	graph := []models.MemorySearchResult{
		{Memory: models.EpisodicMemory{ID: "m1", Summary: "memory 1"}, GraphScore: 1.0},
	}

	merged := rrfMerge(bm25, vector, graph)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged results, got %d", len(merged))
	}

	// m1 rank in streams: bm25 = 1 (1/61), vector = not present, graph = 1 (1/61)
	// m2 rank in streams: bm25 = 2 (1/62), vector = 1 (1/61), graph = not present
	// Verify sorting order
	if merged[0].Memory.ID != "m1" && merged[0].Memory.ID != "m2" {
		t.Errorf("unexpected top merged memory: %s", merged[0].Memory.ID)
	}
}

func TestMemoryService_RecordObservation_Validation(t *testing.T) {
	svc := NewMemoryService(nil, nil)

	_, err := svc.RecordObservation(context.Background(), models.CreateMemoryInput{
		AgentID: "",
		Content: "valid content",
	})
	if err == nil {
		t.Error("expected error for empty agent ID")
	}

	_, err = svc.RecordObservation(context.Background(), models.CreateMemoryInput{
		AgentID: "agent-1",
		Content: "",
	})
	if err == nil {
		t.Error("expected error for empty content")
	}
}
