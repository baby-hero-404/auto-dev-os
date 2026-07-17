package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TestRunPatchRetryLoop_PartialResult_SalvagesSuccessfully verifies that when the tool loop
// signals a partial result (tool_loop_partial=true), runPatchRetryLoop checkpoints the
// worktree, runs targeted tests, and — on success — accepts the salvaged edits instead of
// treating the exhausted iteration budget as a hard failure (Issue 6 / REQ-002).
func TestRunPatchRetryLoop_PartialResult_SalvagesSuccessfully(t *testing.T) {
	task := &models.Task{ID: "task-1"}
	agent := &models.Agent{ID: "agent-1"}

	llmRunner := &mockLLMRunner{
		result: StepResult{
			"status":            "llm_partial",
			"tool_loop_partial": true,
			"edits_applied":     []string{"server/internal/foo.go"},
			"files_read":        []string{"server/internal/foo.go", "server/internal/foo_test.go"},
		},
	}
	worktree := &mockWorktreeManager{}
	diff := &mockDiffCapturer{hostPath: "/repo", changed: []string{"server/internal/foo.go"}}
	tester := &mockTestRunner{result: StepResult{"passed": true}}
	tasks := &mockTaskReader{task: task}
	logger := &mockLogger{}

	cfg := patchRetryConfig{
		Task:       task,
		Agent:      agent,
		JobID:      "job-1",
		StepID:     "code_backend_0",
		TestLabel:  "code_backend_test",
		MaxRetries: 1,
		Agentic:    true,
		LLM:        llmRunner,
		Worktree:   worktree,
		Diff:       diff,
		Tester:     tester,
		Tasks:      tasks,
		Log:        logger,
	}

	out, patchApplied, err := runPatchRetryLoop(context.Background(), cfg, "do the work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !patchApplied {
		t.Error("expected the salvaged partial result to be accepted (patchApplied=true)")
	}
	if out["tool_loop_partial"] != true {
		t.Errorf("expected out to retain tool_loop_partial=true, got %v", out)
	}

	// A salvage checkpoint must be created BEFORE running targeted tests.
	if len(worktree.checkpointStepIDs) != 1 || worktree.checkpointStepIDs[0] != "code_backend_0_salvage" {
		t.Errorf("expected exactly one salvage checkpoint for step 'code_backend_0_salvage', got %v", worktree.checkpointStepIDs)
	}
	if !tester.called {
		t.Error("expected targeted tests to run against the salvaged edits")
	}
	// Tests passed, so no restore should have happened.
	if len(worktree.restoredHashes) != 0 {
		t.Errorf("expected no checkpoint restore on successful test run, got %v", worktree.restoredHashes)
	}
}

// TestRunPatchRetryLoop_PartialResult_RestoresOnTestFailureAndRetries verifies that when the
// salvaged partial result fails targeted tests, the worktree is restored to the salvage
// checkpoint (not lost, not left corrupted) and the retry instruction carries forward a note
// of files already read in the exhausted attempt (Issue 6 retry carry-forward).
func TestRunPatchRetryLoop_PartialResult_RestoresOnTestFailureAndRetries(t *testing.T) {
	task := &models.Task{ID: "task-1"}
	agent := &models.Agent{ID: "agent-1"}

	callCount := 0
	llmRunner := &fakeSequenceLLMRunner{
		results: []StepResult{
			{
				"status":            "llm_partial",
				"tool_loop_partial": true,
				"edits_applied":     []string{"server/internal/foo.go"},
				"files_read":        []string{"server/internal/foo.go", "server/internal/bar.go"},
			},
			{
				"status": "llm_completed",
				"parsed": map[string]any{"summary": "fixed the failing test"},
			},
		},
		onCall: func(instruction string) { callCount++ },
	}
	worktree := &mockWorktreeManager{}
	diff := &mockDiffCapturer{hostPath: "/repo", changed: []string{"server/internal/foo.go"}}
	tester := &sequenceTestRunner{
		results: []testRunOutcome{
			{err: errFakeTestFailure},
			{result: StepResult{"passed": true}},
		},
	}
	tasks := &mockTaskReader{task: task}
	logger := &mockLogger{}

	cfg := patchRetryConfig{
		Task:       task,
		Agent:      agent,
		JobID:      "job-1",
		StepID:     "code_backend_0",
		TestLabel:  "code_backend_test",
		MaxRetries: 3,
		Agentic:    true,
		LLM:        llmRunner,
		Worktree:   worktree,
		Diff:       diff,
		Tester:     tester,
		Tasks:      tasks,
		Log:        logger,
	}

	out, patchApplied, err := runPatchRetryLoop(context.Background(), cfg, "do the work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !patchApplied {
		t.Error("expected the second attempt to succeed (patchApplied=true)")
	}
	if out["status"] != "llm_completed" {
		t.Errorf("expected the final result to be from the second (successful) attempt, got %v", out)
	}

	// One checkpoint created (before the first, failed test run), one restore (after it failed).
	if len(worktree.checkpointStepIDs) != 1 {
		t.Errorf("expected exactly one salvage checkpoint, got %v", worktree.checkpointStepIDs)
	}
	if len(worktree.restoredHashes) != 1 || worktree.restoredHashes[0] != "mock-commit-hash-code_backend_0_salvage" {
		t.Errorf("expected the worktree to be restored to the salvage checkpoint after the failed test run, got %v", worktree.restoredHashes)
	}

	// The retry instruction for the second attempt must carry forward the "already read" note.
	if len(llmRunner.instructions) != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(llmRunner.instructions))
	}
	secondInstruction := llmRunner.instructions[1]
	if !strings.Contains(secondInstruction, "server/internal/foo.go") || !strings.Contains(secondInstruction, "server/internal/bar.go") {
		t.Errorf("expected the retry instruction to carry forward the files read in the prior attempt, got: %s", secondInstruction)
	}
}

// fakeSequenceLLMRunner returns a different StepResult on each successive call, recording the
// instruction text passed each time.
type fakeSequenceLLMRunner struct {
	results      []StepResult
	instructions []string
	onCall       func(instruction string)
	callIdx      int
}

func (m *fakeSequenceLLMRunner) RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (StepResult, error) {
	m.instructions = append(m.instructions, instruction)
	if m.onCall != nil {
		m.onCall(instruction)
	}
	idx := m.callIdx
	if idx >= len(m.results) {
		idx = len(m.results) - 1
	}
	m.callIdx++
	return m.results[idx], nil
}

type testRunOutcome struct {
	result StepResult
	err    error
}

var errFakeTestFailure = &fakeTestFailureError{}

type fakeTestFailureError struct{}

func (e *fakeTestFailureError) Error() string { return "targeted tests failed: assertion mismatch" }

// sequenceTestRunner returns a different outcome on each successive RunTargetedTests call.
type sequenceTestRunner struct {
	results []testRunOutcome
	callIdx int
	called  bool
}

func (m *sequenceTestRunner) RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (StepResult, error) {
	m.called = true
	idx := m.callIdx
	if idx >= len(m.results) {
		idx = len(m.results) - 1
	}
	m.callIdx++
	outcome := m.results[idx]
	return outcome.result, outcome.err
}

// TestRunPatchRetryLoop_ErrNoProgress verifies that when a step fails with zero edits applied,
// the error is wrapped with workflow.ErrNoProgress.
func TestRunPatchRetryLoop_ErrNoProgress(t *testing.T) {
	task := &models.Task{ID: "task-1"}
	agent := &models.Agent{ID: "agent-1"}

	// LLM returns format error / validation error directly
	llmRunner := &mockLLMRunner{
		err: fmt.Errorf("mock LLM failure"),
	}
	worktree := &mockWorktreeManager{}
	diff := &mockDiffCapturer{hostPath: "/repo", changed: nil}
	tester := &mockTestRunner{}
	tasks := &mockTaskReader{task: task}
	logger := &mockLogger{}

	cfg := patchRetryConfig{
		Task:       task,
		Agent:      agent,
		JobID:      "job-1",
		StepID:     "code_backend_0",
		TestLabel:  "code_backend_test",
		MaxRetries: 3,
		Agentic:    true,
		LLM:        llmRunner,
		Worktree:   worktree,
		Diff:       diff,
		Tester:     tester,
		Tasks:      tasks,
		Log:        logger,
	}

	_, _, err := runPatchRetryLoop(context.Background(), cfg, "do the work")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, workflow.ErrNoProgress) {
		t.Errorf("expected ErrNoProgress error, got %v", err)
	}
}

func TestRunPatchRetryLoop_TransientErrorNoWrap(t *testing.T) {
	task := &models.Task{ID: "task-1"}
	agent := &models.Agent{ID: "agent-1"}

	// LLM returns transient error (e.g. timeout / connection refused)
	llmRunner := &mockLLMRunner{
		err: fmt.Errorf("request timeout: gateway unavailable"),
	}
	worktree := &mockWorktreeManager{}
	diff := &mockDiffCapturer{hostPath: "/repo", changed: nil}
	tester := &mockTestRunner{}
	tasks := &mockTaskReader{task: task}
	logger := &mockLogger{}

	cfg := patchRetryConfig{
		Task:       task,
		Agent:      agent,
		JobID:      "job-1",
		StepID:     "code_backend_0",
		TestLabel:  "code_backend_test",
		MaxRetries: 3,
		Agentic:    true,
		LLM:        llmRunner,
		Worktree:   worktree,
		Diff:       diff,
		Tester:     tester,
		Tasks:      tasks,
		Log:        logger,
	}

	_, _, err := runPatchRetryLoop(context.Background(), cfg, "do the work")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if errors.Is(err, workflow.ErrNoProgress) {
		t.Errorf("expected transient error to NOT be wrapped in ErrNoProgress, got %v", err)
	}
}

// TestRunPatchRetryLoop_PartialResult_EmptyCheckpoint verifies that when the tool loop
// signals a partial result, but the resulting Git checkpoint is empty (no real edits),
// the loop treats it as no progress and wraps the final error in ErrNoProgress (T-006).
func TestRunPatchRetryLoop_PartialResult_EmptyCheckpoint(t *testing.T) {
	task := &models.Task{ID: "task-1"}
	agent := &models.Agent{ID: "agent-1"}

	llmRunner := &mockLLMRunner{
		result: StepResult{
			"status":            "llm_partial",
			"tool_loop_partial": true,
			"edits_applied":     []string{"server/internal/foo.go"},
		},
	}
	
	// Mock returns IsEmpty = true
	worktree := &mockWorktreeManager{
		checkpointResult: &models.CheckpointResult{Hash: "mock-hash", IsEmpty: true},
	}
	diff := &mockDiffCapturer{hostPath: "/repo", changed: nil} // No changed files
	tester := &mockTestRunner{}
	tasks := &mockTaskReader{task: task}
	logger := &mockLogger{}

	cfg := patchRetryConfig{
		Task:       task,
		Agent:      agent,
		JobID:      "job-1",
		StepID:     "code_backend_0",
		TestLabel:  "code_backend_test",
		MaxRetries: 1, // Fail fast for test
		Agentic:    true,
		LLM:        llmRunner,
		Worktree:   worktree,
		Diff:       diff,
		Tester:     tester,
		Tasks:      tasks,
		Log:        logger,
	}

	_, _, err := runPatchRetryLoop(context.Background(), cfg, "do the work")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, workflow.ErrNoProgress) {
		t.Errorf("expected error to be wrapped in ErrNoProgress due to empty checkpoint, got %v", err)
	}
}

