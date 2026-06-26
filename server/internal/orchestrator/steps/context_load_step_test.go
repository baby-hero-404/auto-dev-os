package steps

import (
	"context"
	"os"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestContextLoadStep_TransitionsStatusAndGathersContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-step-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-123",
		Status:    models.TaskStatusTodo,
	}
	statusMock := &mockStatusUpdater{}
	artifactMock := &mockArtifactSaver{}
	sandboxMock := &mockSandboxRunner{
		result: StepResult{"stdout": "mock output\n"},
	}

	step := NewContextLoadStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		statusMock,
		&mockStepWorkspaceLoader{},
		sandboxMock,
		&mockLLMChatter{},
		artifactMock,
		&mockRepositoryLister{},
		&mockLogger{},
		func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return "/sandbox/root"
		},
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !statusMock.called {
		t.Error("expected status updater to be called")
	}
	if statusMock.lastStatus != models.TaskStatusContextLoading {
		t.Errorf("expected transition to context loading, got: %s", statusMock.lastStatus)
	}
	if !artifactMock.called {
		t.Error("expected artifact to be saved")
	}

	gitLogs, ok := result["git_logs"].(map[string]string)
	if !ok || gitLogs["root"] != "mock output" {
		t.Errorf("expected git_logs to contain sandbox git output, got: %#v", gitLogs)
	}
}
