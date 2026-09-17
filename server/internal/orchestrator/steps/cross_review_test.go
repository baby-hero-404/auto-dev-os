package steps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func newCrossReviewTestTask() *models.Task {
	return &models.Task{ID: "task-cr-1", ProjectID: "proj-cr-1", Title: "Add feature", Description: "do the thing"}
}

func TestCrossReviewStep_PassingVerdict_ProceedsToHumanReview(t *testing.T) {
	task := newCrossReviewTestTask()
	rt := StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}

	llmMock := &mockCLIReviewRunner{coderRef: "claude_code", reviewerRef: "openai_codex", verdict: map[string]any{
		"spec_compliance": map[string]any{"verdict": "pass"},
		"code_quality":    map[string]any{"verdict": "pass"},
		"summary":         "looks good",
	}}
	status := &mockStatusUpdater{}
	step := NewCrossReviewStep(
		rt, &mockTaskReader{}, &mockProjectReader{project: &models.Project{ReviewHarnessPolicy: models.ReviewHarnessDifferentProvider}},
		llmMock, &mockDiffCapturer{}, &mockWorktreeHostPathResolver{root: t.TempDir()}, &mockArtifactSaver{},
		&mockCheckpointReader{}, &mockCheckpointLister{}, status, &mockLogger{},
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{StepID: workflow.StepCrossReview})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["status"] != nil {
		// status key is optional; just ensure no crash on access
		_ = res["status"]
	}
	if status.lastStatus != models.TaskStatusHumanReview {
		t.Errorf("expected status human_review, got %s", status.lastStatus)
	}
}

func TestCrossReviewStep_FailingVerdict_ReturnsCrossReviewFixLoop(t *testing.T) {
	task := newCrossReviewTestTask()
	rt := StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}

	llmMock := &mockCLIReviewRunner{coderRef: "claude_code", reviewerRef: "openai_codex", verdict: map[string]any{
		"spec_compliance": map[string]any{
			"verdict":    "fail",
			"violations": []any{map[string]any{"requirement": "must validate input", "observed": "no validation", "severity": "high"}},
		},
		"code_quality": map[string]any{"verdict": "pass"},
		"summary":      "missing validation",
	}}
	status := &mockStatusUpdater{}
	step := NewCrossReviewStep(
		rt, &mockTaskReader{}, &mockProjectReader{project: &models.Project{ReviewHarnessPolicy: models.ReviewHarnessDifferentProvider, MaxReviewFixCycles: 3}},
		llmMock, &mockDiffCapturer{}, &mockWorktreeHostPathResolver{root: t.TempDir()}, &mockArtifactSaver{},
		&mockCheckpointReader{count: 0}, &mockCheckpointLister{}, status, &mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{StepID: workflow.StepCrossReview})
	if !errors.Is(err, workflow.ErrCrossReviewFixLoop) {
		t.Fatalf("expected ErrCrossReviewFixLoop, got: %v", err)
	}
	if status.lastStatus != models.TaskStatusCoding {
		t.Errorf("expected status coding, got %s", status.lastStatus)
	}
}

func TestCrossReviewStep_CycleLimitReached_ProceedsDespiteFindings(t *testing.T) {
	task := newCrossReviewTestTask()
	rt := StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}

	llmMock := &mockCLIReviewRunner{coderRef: "claude_code", reviewerRef: "openai_codex", verdict: map[string]any{
		"spec_compliance": map[string]any{"verdict": "fail", "violations": []any{map[string]any{"requirement": "x", "observed": "y", "severity": "low"}}},
		"code_quality":    map[string]any{"verdict": "pass"},
	}}
	status := &mockStatusUpdater{}
	step := NewCrossReviewStep(
		rt, &mockTaskReader{}, &mockProjectReader{project: &models.Project{ReviewHarnessPolicy: models.ReviewHarnessDifferentProvider, MaxReviewFixCycles: 2}},
		llmMock, &mockDiffCapturer{}, &mockWorktreeHostPathResolver{root: t.TempDir()}, &mockArtifactSaver{},
		&mockCheckpointReader{count: 2}, &mockCheckpointLister{}, status, &mockLogger{},
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{StepID: workflow.StepCrossReview})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reached, _ := res["cycle_limit_reached"].(bool); !reached {
		t.Errorf("expected cycle_limit_reached=true, got %v", res["cycle_limit_reached"])
	}
}

// cross_review skips rather than degrading to a self-review when the
// orchestrator reports no second harness — the property the whole
// cross-harness feature exists for.
func TestCrossReviewStep_SingleHarness_SkipsInsteadOfSelfReviewing(t *testing.T) {
	task := newCrossReviewTestTask()
	rt := StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}

	runner := &mockCLIReviewRunner{err: ErrNoDistinctCLIHarness}
	status := &mockStatusUpdater{}
	logger := &mockLogger{}
	step := NewCrossReviewStep(
		rt, &mockTaskReader{}, &mockProjectReader{project: &models.Project{ReviewHarnessPolicy: models.ReviewHarnessDifferentProvider}},
		runner, &mockDiffCapturer{}, &mockWorktreeHostPathResolver{root: t.TempDir()}, &mockArtifactSaver{},
		&mockCheckpointReader{}, &mockCheckpointLister{}, status, logger,
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{StepID: workflow.StepCrossReview})
	if err != nil {
		t.Fatalf("a missing second harness must not fail the job: %v", err)
	}
	if res["status"] != "skipped" || res["skipped_reason"] != "single_harness" {
		t.Errorf("expected a skipped result naming the reason, got %#v", res)
	}
	if status.lastStatus != models.TaskStatusHumanReview {
		t.Errorf("expected the task to still reach human_review, got %s", status.lastStatus)
	}
	var logged bool
	for _, m := range logger.messages {
		if strings.Contains(m, "only one CLI harness available") {
			logged = true
		}
	}
	if !logged {
		t.Errorf("expected the skip to be logged so the lost review is visible, got %v", logger.messages)
	}
}

// review_harness_policy="same" is the one case where reusing the coder's
// harness is the configured intent, and it must be recorded as a fallback.
func TestCrossReviewStep_PolicySame_AllowsSameHarnessAndFlagsIt(t *testing.T) {
	task := newCrossReviewTestTask()
	rt := StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}

	runner := &mockCLIReviewRunner{
		coderRef:    "claude_code",
		reviewerRef: "claude_code",
		verdict: map[string]any{
			"spec_compliance": map[string]any{"verdict": "pass"},
			"code_quality":    map[string]any{"verdict": "pass"},
		},
	}
	step := NewCrossReviewStep(
		rt, &mockTaskReader{}, &mockProjectReader{project: &models.Project{ReviewHarnessPolicy: models.ReviewHarnessSame}},
		runner, &mockDiffCapturer{}, &mockWorktreeHostPathResolver{root: t.TempDir()}, &mockArtifactSaver{},
		&mockCheckpointReader{}, &mockCheckpointLister{}, &mockStatusUpdater{}, &mockLogger{},
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{StepID: workflow.StepCrossReview})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !runner.lastAllowSame {
		t.Error("expected allowSameHarness to be passed through for policy=same")
	}
	if res["self_review_fallback"] != true {
		t.Errorf("expected a self-review fallback flag when coder and reviewer match, got %#v", res)
	}
}

func TestCrossReviewFeedback_FormatsViolationsFromMostRecentCheckpoint(t *testing.T) {
	state, _ := json.Marshal(map[string]any{
		"output": map[string]any{
			"review_verdict": map[string]any{
				"spec_compliance": map[string]any{
					"violations": []any{map[string]any{"requirement": "must validate input", "observed": "no validation", "severity": "high"}},
				},
				"code_quality": map[string]any{},
			},
		},
	})
	lister := &mockCheckpointLister{cps: []models.WorkflowCheckpoint{
		{Step: workflow.StepCrossReview, State: state},
	}}

	feedback := crossReviewFeedback(context.Background(), lister, "task-1")
	if feedback == "" {
		t.Fatal("expected non-empty feedback")
	}
}
