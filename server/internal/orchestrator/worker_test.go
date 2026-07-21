package orchestrator

import (
	"testing"
	"time"
)

// TestDueForLearningNudge_FiresEveryIntervalSteps simulates 8 successful
// workflow steps against the same interval (4) worker.go's OnEvent uses,
// and verifies the mid-task nudge fires exactly twice (after steps 4 and 8)
// — the exact cadence described in
// docs/openspecs/agent-prompt-management-enhancements/tasks.md Task 2.1.
func TestDueForLearningNudge_FiresEveryIntervalSteps(t *testing.T) {
	const interval = 4
	fireCount := 0
	successStepCount := 0

	for range 8 {
		successStepCount++
		if dueForLearningNudge(successStepCount, interval) {
			fireCount++
		}
	}

	if fireCount != 2 {
		t.Errorf("expected DetectPatterns nudge to fire exactly twice across 8 steps, got %d", fireCount)
	}
}

func TestDueForLearningNudge_NeverFiresOnZeroSteps(t *testing.T) {
	if dueForLearningNudge(0, 4) {
		t.Error("expected no nudge before any successful step has completed")
	}
}

func TestDueForLearningNudge_DisabledWhenIntervalIsZero(t *testing.T) {
	if dueForLearningNudge(4, 0) {
		t.Error("expected no nudge when interval is non-positive")
	}
}

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
