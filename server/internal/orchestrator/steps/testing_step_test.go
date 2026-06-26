package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestTestStep_ExecutesSuccessfully(t *testing.T) {
	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-1",
	}

	sandboxMock := &mockSandboxRunner{
		result: StepResult{
			"stdout": "LINT_STATUS: PASSED\nBUILD_STATUS: PASSED\n",
		},
	}

	statusMock := &mockStatusUpdater{}
	artifactMock := &mockArtifactSaver{}

	step := NewTestStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		statusMock,
		sandboxMock,
		&mockStepWorkspaceLoader{},
		&mockProjectReader{project: &models.Project{ID: "proj-1", MaxReviewFixCycles: 3}},
		&mockCheckpointReader{count: 0},
		artifactMock,
		&mockLogger{},
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["lint_status"] != "passed" {
		t.Errorf("expected lint_status passed, got: %v", result["lint_status"])
	}
	if result["build_status"] != "passed" {
		t.Errorf("expected build_status passed, got: %v", result["build_status"])
	}
	if statusMock.lastStatus != models.TaskStatusTesting {
		t.Errorf("expected status to transition to testing, got: %s", statusMock.lastStatus)
	}
}

func TestTestStep_LoopsBackToReviewFix(t *testing.T) {
	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-1",
	}

	sandboxMock := &mockSandboxRunner{
		err: errors.New("compilation failed"),
	}

	statusMock := &mockStatusUpdater{}

	step := NewTestStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		statusMock,
		sandboxMock,
		&mockStepWorkspaceLoader{},
		&mockProjectReader{project: &models.Project{ID: "proj-1", MaxReviewFixCycles: 3}},
		&mockCheckpointReader{count: 1}, // 1 successful cycle, less than MaxReviewFixCycles (3)
		&mockArtifactSaver{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if !errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Errorf("expected ErrReviewFixLoop, got: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusReviewing {
		t.Errorf("expected status to transition to reviewing (loop back), got: %s", statusMock.lastStatus)
	}
}

func TestTestStep_FailsWhenLimitReached(t *testing.T) {
	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-1",
	}

	sandboxMock := &mockSandboxRunner{
		err: errors.New("compilation failed"),
	}

	statusMock := &mockStatusUpdater{}

	step := NewTestStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		statusMock,
		sandboxMock,
		&mockStepWorkspaceLoader{},
		&mockProjectReader{project: &models.Project{ID: "proj-1", MaxReviewFixCycles: 3}},
		&mockCheckpointReader{count: 3}, // 3 successful cycles, matches MaxReviewFixCycles
		&mockArtifactSaver{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Error("expected original error, got ErrReviewFixLoop")
	}
}
