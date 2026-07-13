package checkpoint

import (
	"context"
	"encoding/json"
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

type recoveryArtifactRepo struct {
	artifacts []models.WorkflowArtifact
}

func (r *recoveryArtifactRepo) Create(ctx context.Context, artifact *models.WorkflowArtifact) error {
	r.artifacts = append(r.artifacts, *artifact)
	return nil
}

func (r *recoveryArtifactRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error) {
	return r.artifacts, nil
}

func TestGetLatestExecutionSnapshot_ReturnsLatest(t *testing.T) {
	snap1 := models.ExecutionSnapshot{ExecutionID: "step-1", CurrentState: "IMPLEMENTATION", PromptHash: "hash-1"}
	payload1, _ := json.Marshal(snap1)
	snap2 := models.ExecutionSnapshot{ExecutionID: "step-1", CurrentState: "VALIDATION", PromptHash: "hash-2"}
	payload2, _ := json.Marshal(snap2)

	repo := &recoveryArtifactRepo{
		artifacts: []models.WorkflowArtifact{
			{TaskID: "task-1", Step: "step-1", Type: "execution_snapshot", Payload: payload1},
			{TaskID: "task-1", Step: "step-1_cycle_2", Type: "execution_snapshot", Payload: payload2},
		},
	}

	store := &Store{Artifacts: repo}
	gotSnap, exists := store.GetLatestExecutionSnapshot(context.Background(), "task-1", "step-1")
	if !exists {
		t.Fatal("expected snapshot to exist")
	}
	if gotSnap.PromptHash != "hash-2" {
		t.Errorf("expected latest snapshot hash-2, got %s", gotSnap.PromptHash)
	}
	if gotSnap.CurrentState != "VALIDATION" {
		t.Errorf("expected state VALIDATION, got %s", gotSnap.CurrentState)
	}
}

func TestSaveArtifact_AutoCyclesStepName(t *testing.T) {
	repo := &recoveryArtifactRepo{}
	store := &Store{Artifacts: repo}

	err := store.SaveArtifact(context.Background(), "job-1", "task-1", "step-1", "execution_snapshot", models.ExecutionSnapshot{PromptHash: "hash-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = store.SaveArtifact(context.Background(), "job-1", "task-1", "step-1", "execution_snapshot", models.ExecutionSnapshot{PromptHash: "hash-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(repo.artifacts))
	}
	if repo.artifacts[0].Step != "step-1" {
		t.Errorf("expected first artifact step step-1, got %s", repo.artifacts[0].Step)
	}
	if repo.artifacts[1].Step != "step-1_cycle_2" {
		t.Errorf("expected second artifact step step-1_cycle_2, got %s", repo.artifacts[1].Step)
	}
}
