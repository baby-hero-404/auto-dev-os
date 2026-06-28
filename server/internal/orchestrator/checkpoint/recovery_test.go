package checkpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type recoveryWorkflowRepo struct {
	checkpoint models.WorkflowCheckpoint
}

func (r recoveryWorkflowRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	return []models.WorkflowCheckpoint{r.checkpoint}, nil
}

func TestWithCheckpointRecovery_RestoresStatusOnResume(t *testing.T) {
	updated := ""
	store := &Store{
		Workflows: recoveryWorkflowRepo{checkpoint: models.WorkflowCheckpoint{TaskID: "task-1", Step: workflow.StepCodeBackend, State: []byte(`{"status":"success","output":{"status":"ok"}}`)}},
		Log:       func(ctx context.Context, taskID string, jobID *string, level string, message string) {},
	}
	runnerCalled := false
	runner := func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		runnerCalled = true
		return nil, nil
	}
	wrapped := store.WithCheckpointRecovery(
		workflow.StepCodeBackend,
		func(output map[string]any) string { return models.TaskStatusCoding },
		&models.Task{ID: "task-1"},
		&models.Agent{ID: "agent-1"},
		workflow.StepPlan,
		runner,
		nil,
		func(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
			updated = newStatus
			return &models.Task{ID: taskID, Status: newStatus}, nil
		},
	)

	res, err := wrapped(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("wrapped returned error: %v", err)
	}
	if runnerCalled {
		t.Fatal("expected runner to be skipped on successful checkpoint")
	}
	if updated != models.TaskStatusCoding {
		t.Fatalf("expected status restore to coding, got %q", updated)
	}
	if res["status"] != "ok" {
		t.Fatalf("expected checkpoint output to be returned, got %v", res)
	}
}

func TestWithCheckpointRecovery_IgnoresStatusRestoreError(t *testing.T) {
	store := &Store{
		Workflows: recoveryWorkflowRepo{checkpoint: models.WorkflowCheckpoint{TaskID: "task-1", Step: workflow.StepCodeBackend, State: []byte(`{"status":"success","output":{"status":"ok"}}`)}},
		Log:       func(ctx context.Context, taskID string, jobID *string, level string, message string) {},
	}
	wrapped := store.WithCheckpointRecovery(
		workflow.StepCodeBackend,
		func(output map[string]any) string { return models.TaskStatusCoding },
		&models.Task{ID: "task-1"},
		&models.Agent{ID: "agent-1"},
		workflow.StepPlan,
		func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
			return nil, errors.New("runner should not be called")
		},
		nil,
		func(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
			return nil, errors.New("transition denied")
		},
	)

	_, err := wrapped(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("expected no error even when status restore fails, got %v", err)
	}
}
