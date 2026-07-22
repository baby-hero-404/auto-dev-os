package steps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestFixStep_SkipsEasyTask(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityEasy}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, &mockWorktreeManager{},
		&mockAffectedFileReader{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got: %v", result["status"])
	}
}

func TestFixStep_SkipsOnCycleLimit(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, &mockWorktreeManager{},
		&mockAffectedFileReader{},
	)
	sc := workflow.StepContext{
		Inputs: map[string]StepResult{
			workflow.StepReview: {"cycle_limit_reached": true},
		},
	}
	result, err := step.Execute(context.Background(), sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

func TestFixStep_SkipsOnNoFindings(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockCheckpointLister{}, // No PR rejection feedback
		nil, nil, nil, nil, nil, nil, nil, &mockWorktreeManager{},
		&mockAffectedFileReader{},
	)
	sc := workflow.StepContext{
		Inputs: map[string]StepResult{
			workflow.StepReview: {
				"parsed": map[string]any{
					"findings": []any{},
				},
			},
		},
	}
	result, err := step.Execute(context.Background(), sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

func TestFixStep_AppliesFixAndTriggersLoop(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	patchMock := &mockPatchApplier{}
	testMock := &mockTestRunner{}
	statusMock := &mockStatusUpdater{}
	artifactMock := &mockArtifactSaver{}
	diffMock := &mockDiffCapturer{
		diffVal:  "new diff",
		hostPath: "/host/repo",
		changed:  []string{"main.go"},
	}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockCheckpointLister{},
		&mockLLMRunner{result: StepResult{
			"parsed": map[string]any{
				"summary": "fixed the finding via tool calls",
			},
		}},
		diffMock,
		artifactMock,
		patchMock,
		testMock,
		statusMock,
		&mockLogger{},
		&mockWorktreeManager{},
		&mockAffectedFileReader{},
	)
	sc := workflow.StepContext{
		Inputs: map[string]StepResult{
			workflow.StepReview: {
				"parsed": map[string]any{
					"findings": []any{map[string]any{"file": "main.go", "severity": "high"}},
				},
			},
		},
	}
	_, err := step.Execute(context.Background(), sc)
	if !errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Fatalf("expected ErrReviewFixLoop, got: %v", err)
	}
	// Agentic mode: edits are applied via tool calls inside the LLM call itself, so the
	// diff-based patch applier is no longer invoked — only the post-hoc test verification is.
	if !testMock.called {
		t.Error("expected targeted tests to be run")
	}
	if statusMock.lastStatus != models.TaskStatusReviewing {
		t.Errorf("expected status to transition to reviewing, got: %s", statusMock.lastStatus)
	}
}

func TestFixStep_StructuredVerdict_ViolationsFirstInstruction(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	llmMock := &mockLLMRunner{result: StepResult{"parsed": map[string]any{"summary": "fixed"}}}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockCheckpointLister{},
		llmMock,
		&mockDiffCapturer{diffVal: "some diff", hostPath: "/host/repo", changed: []string{"main.go"}},
		&mockArtifactSaver{},
		&mockPatchApplier{},
		&mockTestRunner{},
		&mockStatusUpdater{},
		&mockLogger{},
		&mockWorktreeManager{},
		&mockAffectedFileReader{},
	)
	sc := workflow.StepContext{
		Inputs: map[string]StepResult{
			workflow.StepReview: {
				"review_verdict": map[string]any{
					"spec_compliance": map[string]any{
						"verdict": "fail",
						"violations": []any{
							map[string]any{"requirement": "must validate input", "observed": "no validation"},
						},
					},
					"code_quality": map[string]any{
						"verdict": "fail",
						"issues": []any{
							map[string]any{"file": "main.go", "issue": "unused var", "suggestion": "remove it"},
						},
					},
				},
				// Legacy "parsed" findings should be ignored once a structured verdict is present.
				"parsed": map[string]any{
					"findings": []any{map[string]any{"file": "other.go", "severity": "high"}},
				},
			},
		},
	}
	_, _ = step.Execute(context.Background(), sc)
	if !strings.Contains(llmMock.lastInstruction, "## Spec violations (MUST fix first)") {
		t.Errorf("expected violations-first section in instruction, got:\n%s", llmMock.lastInstruction)
	}
	if !strings.Contains(llmMock.lastInstruction, "must validate input") {
		t.Errorf("expected spec violation requirement text in instruction, got:\n%s", llmMock.lastInstruction)
	}
	if !strings.Contains(llmMock.lastInstruction, "## Quality issues") {
		t.Errorf("expected quality issues section in instruction, got:\n%s", llmMock.lastInstruction)
	}
	if strings.Contains(llmMock.lastInstruction, "other.go") {
		t.Errorf("expected legacy findings to be ignored when structured verdict is present, got:\n%s", llmMock.lastInstruction)
	}
}

func TestFixStep_PRDiffFallbackUsesFrontendWorktreeSuffix(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	diffMock := &mockDiffCapturer{}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleFrontend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockCheckpointLister{},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{"summary": "done"}}},
		diffMock,
		nil, nil, nil, nil,
		&mockLogger{},
		&mockWorktreeManager{},
		&mockAffectedFileReader{},
	)
	sc := workflow.StepContext{
		Inputs: map[string]StepResult{
			workflow.StepReview: {
				"parsed": map[string]any{
					"findings": []any{map[string]any{"file": "main.go", "severity": "high"}},
				},
			},
		},
	}

	_, err := step.Execute(context.Background(), sc)
	if err != nil && !errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, suffix := range diffMock.workspaceSuffixes {
		if suffix == "-fe-worktree" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected frontend worktree suffix fallback, got suffix calls %#v", diffMock.workspaceSuffixes)
	}
}

func TestFixStep_PathCanonicalization(t *testing.T) {
	task := &models.Task{
		ID:         "t1",
		Complexity: models.TaskComplexityMedium,
		Analysis: []byte(`{
			"execution_boundaries": [
				{
					"repo_name": "tool_zentao"
				}
			]
		}`),
	}

	llmMock := &mockLLMRunner{
		result: StepResult{
			"parsed": map[string]any{
				"fixes_applied": true,
				"summary":       "done",
			},
		},
	}

	loggerMock := &mockLogger{}

	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockCheckpointLister{},
		llmMock,
		&mockDiffCapturer{},
		nil,
		&mockPatchApplier{},
		&mockTestRunner{},
		&mockStatusUpdater{},
		loggerMock,
		&mockWorktreeManager{},
		&mockAffectedFileReader{},
	)

	// Context with findings:
	// 1. One workspace-prefixed path under the target repo (tool_zentao)
	// 2. One foreign repo path (other_repo) - should be dropped
	sc := workflow.StepContext{
		Inputs: map[string]StepResult{
			workflow.StepReview: {
				"parsed": map[string]any{
					"findings": []any{
						map[string]any{
							"file":     "code/repos/tool_zentao/main/cmd/sync/main.go",
							"severity": "high",
						},
						map[string]any{
							"file":     "code/repos/other_repo/main/cmd/sync/main.go",
							"severity": "high",
						},
					},
				},
			},
		},
	}

	_, err := step.Execute(context.Background(), sc)
	if err != nil && !errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Fatalf("unexpected error: %v", err)
	}

	capturedInstruction := llmMock.lastInstruction

	// Verify that the instruction contains the canonical path cmd/sync/main.go
	if !strings.Contains(capturedInstruction, "cmd/sync/main.go") {
		t.Errorf("expected instruction to contain canonicalized path 'cmd/sync/main.go', got: %s", capturedInstruction)
	}

	// Verify that the instruction does NOT contain the uncanonicalized workspace prefix for tool_zentao
	if strings.Contains(capturedInstruction, "code/repos/tool_zentao/main") {
		t.Errorf("expected instruction to NOT contain workspace path prefix, got: %s", capturedInstruction)
	}

	// Verify that the foreign repo path other_repo was dropped
	if strings.Contains(capturedInstruction, "other_repo") {
		t.Errorf("expected instruction to NOT contain foreign repo path 'other_repo', got: %s", capturedInstruction)
	}

	// Verify that a warning was logged for dropping the foreign repo path
	foundWarning := false
	for _, msg := range loggerMock.messages {
		if strings.Contains(msg, "dropping unresolvable review finding path") && strings.Contains(msg, "other_repo") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected warning to be logged for the dropped foreign repo path, logs: %v", loggerMock.messages)
	}
}
