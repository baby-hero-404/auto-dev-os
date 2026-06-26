package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestFixStep_SkipsEasyTask(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityEasy}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil,
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
		nil, nil, nil, nil, nil, nil, nil, nil,
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
		nil, nil, nil, nil, nil, nil, nil,
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
				"patch": "diff --git a/main.go b/main.go\n...",
			},
		}},
		diffMock,
		artifactMock,
		patchMock,
		testMock,
		statusMock,
		&mockLogger{},
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
	if !patchMock.called {
		t.Error("expected patch to be applied")
	}
	if !testMock.called {
		t.Error("expected targeted tests to be run")
	}
	if statusMock.lastStatus != models.TaskStatusReviewing {
		t.Errorf("expected status to transition to reviewing, got: %s", statusMock.lastStatus)
	}
}

func TestFixStep_PRDiffFallbackUsesFrontendWorktreeSuffix(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	diffMock := &mockDiffCapturer{}
	step := NewFixStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleFrontend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockCheckpointLister{},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{}}},
		diffMock,
		nil, nil, nil, nil,
		&mockLogger{},
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
	if err != nil {
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
