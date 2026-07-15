package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestParseReviewFindings_FieldMapping(t *testing.T) {
	parsed := map[string]any{
		"findings": []any{
			map[string]any{
				"repo":           "tool_zentao",
				"file":           "cmd/sync/main.go",
				"line":           float64(12),
				"severity":       "high",
				"recommendation": "fix it",
				"requires_fix":   true,
			},
		},
	}
	findings, err := ParseReviewFindings(parsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Repo != "tool_zentao" || f.File != "cmd/sync/main.go" || f.Line != 12 ||
		f.Severity != "high" || f.Recommendation != "fix it" || !f.RequiresFix {
		t.Errorf("unexpected finding: %+v", f)
	}
}

func TestReviewStep_SkipsEasyTask(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityEasy}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

func TestReviewStep_BypassedViaHumanReview(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusFixing}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "bypassed_via_human_review" {
		t.Errorf("expected bypassed_via_human_review, got: %v", result["status"])
	}
}

func TestReviewStep_ErrWaitingApproval(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusHumanReview}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Errorf("expected ErrWaitingApproval, got: %v", err)
	}
}

func TestReviewStep_WithFindings(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	artifactMock := &mockArtifactSaver{}
	assigner := &mockReviewerAssigner{agent: &models.Agent{ID: "a2", Name: "Reviewer"}}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "high"}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		artifactMock,
		assigner,
		&mockCheckpointReader{count: 0},
		nil,
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !statusMock.called {
		t.Error("expected status updater to be called")
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected status to transition to fixing, got: %s", statusMock.lastStatus)
	}
	if !artifactMock.called {
		t.Error("expected artifact mock to be called")
	}
	if len(assigner.releasedIDs) != 1 {
		t.Fatalf("expected reviewer agent release, got %d releases", len(assigner.releasedIDs))
	}
	if assigner.releasedIDs[0] != "a2" {
		t.Fatalf("expected reviewer release of a2, got %s", assigner.releasedIDs[0])
	}
	if result["parsed"] == nil {
		t.Error("expected parsed result to be returned")
	}
}

// TestReviewStep_RequiresFixTrue regression-tests the legacy requires_fix:true boolean signal
// (independent of severity), lost when ParseReviewFindings/hasActionableFindings were rewritten
// to the typed models.ReviewFinding contract (Part 2 review finding).
func TestReviewStep_RequiresFixTrue(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "requires_fix": true}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected requires_fix:true (no severity) to route to fixing, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_ExceedsCycleLimit(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "high"}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1", MaxReviewFixCycles: 2}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 2}, // Already ran 2 cycles
		nil,
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusTesting {
		t.Errorf("expected cycle limit to redirect to testing, got: %s", statusMock.lastStatus)
	}
	if result["cycle_limit_reached"] != true {
		t.Error("expected cycle_limit_reached to be true in output")
	}
}

func TestReviewStep_NoActionableFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings map[string]any
	}{
		{
			"missing severity",
			map[string]any{"findings": []any{map[string]any{"file": "main.go"}}},
		},
		{
			"low severity",
			map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "low"}}},
		},
		{
			"info severity",
			map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "info"}}},
		},
		{
			"requires_fix false",
			map[string]any{"findings": []any{map[string]any{"file": "main.go", "requires_fix": false}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
			statusMock := &mockStatusUpdater{}
			step := NewReviewStep(
				StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
				&mockTaskReader{task: task},
				&mockProjectReader{project: &models.Project{ID: "p1"}},
				&mockLLMRunner{result: StepResult{"parsed": tc.findings}},
				&mockDiffCapturer{diffVal: "some diff"},
				&mockArtifactSaver{},
				nil,
				&mockCheckpointReader{count: 0},
				nil,
				statusMock,
				&mockLogger{},
			)
			_, err := step.Execute(context.Background(), workflow.StepContext{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if statusMock.lastStatus != models.TaskStatusTesting {
				t.Errorf("expected status to transition to testing for non-actionable, got: %s", statusMock.lastStatus)
			}
		})
	}
}

func TestReviewStep_SingleFindingFallback(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	// Top-level map acts as a single finding
	singleFinding := map[string]any{"file": "main.go", "severity": "high"}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": singleFinding}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected single finding fallback to transition to fixing, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_PRDiffFallbackUsesBackendWorktreeSuffix(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	diffMock := &mockDiffCapturer{}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleBackend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{"findings": []any{}}}},
		diffMock,
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		&mockStatusUpdater{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diffMock.lastWorkspaceSuffix != "-be-worktree" {
		t.Fatalf("expected backend worktree suffix fallback, got %q", diffMock.lastWorkspaceSuffix)
	}
}
