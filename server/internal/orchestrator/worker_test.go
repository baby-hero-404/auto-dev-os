package orchestrator

import (
	"testing"
	"time"
)

func TestOrchestrator_CalculateBackoff(t *testing.T) {
	orch := &Orchestrator{}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{attempt: 1, expected: 4 * time.Second},  // 2^1 * 2 = 4
		{attempt: 2, expected: 8 * time.Second},  // 2^2 * 2 = 8
		{attempt: 3, expected: 16 * time.Second}, // 2^3 * 2 = 16
		{attempt: 4, expected: 32 * time.Second}, // 2^4 * 2 = 32
		{attempt: 5, expected: 60 * time.Second}, // 2^5 * 2 = 64 -> cap at 60
		{attempt: 6, expected: 60 * time.Second}, // 2^6 * 2 = 128 -> cap at 60
	}

	for _, tc := range tests {
		got := orch.calculateBackoff(tc.attempt)
		if got != tc.expected {
			t.Errorf("calculateBackoff(%d) = %v; want %v", tc.attempt, got, tc.expected)
		}
	}
}
