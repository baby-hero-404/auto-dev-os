package orchestrator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockProjectRepo struct {
	project *models.Project
}

func (m *mockProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return m.project, nil
}

type mockTaskEventCounter struct {
	count int64
}

func (m *mockTaskEventCounter) CountByTaskID(ctx context.Context, taskID string) (int64, error) {
	return m.count, nil
}

type fakeEventEmitter struct {
	events []models.TaskEvent
}

func (f *fakeEventEmitter) Emit(ctx context.Context, taskID, eventType string, schemaVersion int, payload json.RawMessage, artifactID *string) (*models.TaskEvent, error) {
	e := models.TaskEvent{TaskID: taskID, Type: eventType, SchemaVersion: schemaVersion, Payload: payload, ArtifactID: artifactID}
	f.events = append(f.events, e)
	return &e, nil
}

func newGuardrailTestOrchestrator(t *testing.T, task *models.Task, project *models.Project, eventCount int64) (*Orchestrator, *mockTaskRepo) {
	t.Helper()
	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{}
	agentMgr := &mockAgentAssigner{agent: &models.Agent{ID: "agent-1"}}
	runtime := &mockSandboxRuntime{}
	adapter := sandbox.NewEventAdapter(&fakeEventEmitter{}, nil)
	o := New(taskRepo, workflowRepo, agentMgr, runtime,
		WithProjectRepository(&mockProjectRepo{project: project}),
		WithTaskEventCounter(&mockTaskEventCounter{count: eventCount}),
		WithEventAdapter(adapter),
	)
	return o, taskRepo
}

func testProject() *models.Project {
	return &models.Project{
		ID:                  "project-1",
		MaxTaskRetryCount:   5,
		MaxExecutionMinutes: 120,
		MaxEventCount:       20000,
	}
}

// 1. Five consecutive fixing<-testing/reviewing failures trip max_retries_exceeded.
func TestApplyGuardrails_MaxRetriesExceeded(t *testing.T) {
	task := &models.Task{ID: "task-1", ProjectID: "project-1", Status: models.TaskStatusTesting, RetryCount: 4}
	o, taskRepo := newGuardrailTestOrchestrator(t, task, testProject(), 0)

	updated, err := o.updateTaskStatus(context.Background(), "task-1", models.TaskStatusFixing)
	if err != nil {
		t.Fatalf("updateTaskStatus: %v", err)
	}
	if updated.Status != models.TaskStatusBlocked {
		t.Fatalf("expected status blocked, got %q", updated.Status)
	}
	if taskRepo.task.BlockReason != "max_retries_exceeded" {
		t.Fatalf("expected block reason max_retries_exceeded, got %q", taskRepo.task.BlockReason)
	}
}

// 2. A task that fails once, succeeds, then fails four more times does not trip
// the guardrail — retry_count resets on a successful testing exit.
func TestApplyGuardrails_RetryCountResetsOnSuccess(t *testing.T) {
	task := &models.Task{ID: "task-1", ProjectID: "project-1", Status: models.TaskStatusTesting, RetryCount: 1}
	o, taskRepo := newGuardrailTestOrchestrator(t, task, testProject(), 0)

	updated, err := o.updateTaskStatus(context.Background(), "task-1", models.TaskStatusPrReady)
	if err != nil {
		t.Fatalf("updateTaskStatus: %v", err)
	}
	if updated.RetryCount != 0 {
		t.Fatalf("expected retry_count reset to 0, got %d", updated.RetryCount)
	}

	taskRepo.task.Status = models.TaskStatusTesting
	for i := 0; i < 4; i++ {
		taskRepo.task.Status = models.TaskStatusTesting
		updated, err = o.updateTaskStatus(context.Background(), "task-1", models.TaskStatusFixing)
		if err != nil {
			t.Fatalf("updateTaskStatus: %v", err)
		}
		if updated.Status == models.TaskStatusBlocked {
			t.Fatalf("guardrail tripped early on retry %d", i+1)
		}
	}
	if updated.RetryCount != 4 {
		t.Fatalf("expected retry_count 4, got %d", updated.RetryCount)
	}
}

// 3. Execution time past MaxExecutionMinutes trips execution_timeout.
func TestApplyGuardrails_ExecutionTimeout(t *testing.T) {
	startedAt := time.Now().Add(-200 * time.Minute)
	task := &models.Task{ID: "task-1", ProjectID: "project-1", Status: models.TaskStatusCoding, ExecutionStartedAt: &startedAt}
	o, taskRepo := newGuardrailTestOrchestrator(t, task, testProject(), 0)

	updated, err := o.updateTaskStatus(context.Background(), "task-1", models.TaskStatusReviewing)
	if err != nil {
		t.Fatalf("updateTaskStatus: %v", err)
	}
	if updated.Status != models.TaskStatusBlocked {
		t.Fatalf("expected status blocked, got %q", updated.Status)
	}
	if taskRepo.task.BlockReason != "execution_timeout" {
		t.Fatalf("expected block reason execution_timeout, got %q", taskRepo.task.BlockReason)
	}
}

// 4. With cost data unavailable, EvalCostBudget never trips (permanently inactive).
func TestApplyGuardrails_CostBudgetInactive(t *testing.T) {
	project := testProject()
	budget := 0.01
	project.CostBudget = &budget
	task := &models.Task{ID: "task-1", ProjectID: "project-1", Status: models.TaskStatusCoding}
	o, taskRepo := newGuardrailTestOrchestrator(t, task, project, 0)

	updated, err := o.updateTaskStatus(context.Background(), "task-1", models.TaskStatusReviewing)
	if err != nil {
		t.Fatalf("updateTaskStatus: %v", err)
	}
	if updated.Status == models.TaskStatusBlocked {
		t.Fatalf("cost budget guardrail must never trip, got blocked with reason %q", taskRepo.task.BlockReason)
	}
}

// 5. Crossing MaxEventCount task_events rows blocks the task with event_volume_exceeded.
func TestCheckEventVolumeGuardrail_TripsOverLimit(t *testing.T) {
	task := &models.Task{ID: "task-1", ProjectID: "project-1", Status: models.TaskStatusCoding}
	o, taskRepo := newGuardrailTestOrchestrator(t, task, testProject(), 20000)

	o.checkEventVolumeGuardrail(context.Background(), "task-1")

	if taskRepo.task.Status != models.TaskStatusBlocked {
		t.Fatalf("expected status blocked, got %q", taskRepo.task.Status)
	}
	if taskRepo.task.BlockReason != "event_volume_exceeded" {
		t.Fatalf("expected block reason event_volume_exceeded, got %q", taskRepo.task.BlockReason)
	}
}

// 7. A normal task under every threshold completes without an unexpected
// blocked transition (no false positives on the happy path).
func TestApplyGuardrails_HappyPathNoFalsePositive(t *testing.T) {
	task := &models.Task{ID: "task-1", ProjectID: "project-1", Status: models.TaskStatusTodo}
	o, taskRepo := newGuardrailTestOrchestrator(t, task, testProject(), 5)

	updated, err := o.updateTaskStatus(context.Background(), "task-1", models.TaskStatusCoding)
	if err != nil {
		t.Fatalf("updateTaskStatus: %v", err)
	}
	if updated.Status == models.TaskStatusBlocked {
		t.Fatalf("unexpected block on happy path: %q", taskRepo.task.BlockReason)
	}
	if updated.ExecutionStartedAt == nil {
		t.Fatalf("expected execution_started_at to be set on first leaving todo")
	}

	o.checkEventVolumeGuardrail(context.Background(), "task-1")
	if taskRepo.task.Status == models.TaskStatusBlocked {
		t.Fatalf("unexpected block from event volume check on happy path")
	}
}
