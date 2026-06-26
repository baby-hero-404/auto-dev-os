package steps

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestPlanStep_SkipsEasyTask(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityEasy}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil,
		&mockStatusUpdater{},
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got: %v", result["status"])
	}
}

func TestPlanStep_TransitionsToCoding(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
	statusMock := &mockStatusUpdater{}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{result: StepResult{"subtasks": []any{}}},
		nil, nil, statusMock, &mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !statusMock.called {
		t.Error("expected status mock to be called")
	}
	if statusMock.lastStatus != models.TaskStatusCoding {
		t.Errorf("expected status to transition to coding, got: %s", statusMock.lastStatus)
	}
}
