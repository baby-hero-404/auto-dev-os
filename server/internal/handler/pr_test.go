package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

// Define mock repositories for Orchestrator
type testTaskRepo struct {
	task *models.Task
}

func (m *testTaskRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	if m.task != nil && m.task.ID == id {
		return m.task, nil
	}
	return nil, errors.New("task not found")
}

func (m *testTaskRepo) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	if m.task != nil && m.task.ID == id {
		if input.Status != nil {
			m.task.Status = *input.Status
		}
		return m.task, nil
	}
	return nil, errors.New("task not found")
}

type testWorkflowRepo struct {
	job         *models.WorkflowJob
	checkpoints []models.WorkflowCheckpoint
}

func (m *testWorkflowRepo) Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	m.job = &models.WorkflowJob{ID: "job-123", TaskID: taskID, Status: models.WorkflowJobStatusQueued}
	return m.job, nil
}

func (m *testWorkflowRepo) ClaimNext(ctx context.Context) (*models.WorkflowJob, error) {
	return nil, nil
}

func (m *testWorkflowRepo) LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	if m.job != nil && m.job.TaskID == taskID {
		return m.job, nil
	}
	return nil, errors.New("job not found")
}

func (m *testWorkflowRepo) UpdateJob(ctx context.Context, id string, updates map[string]any) (*models.WorkflowJob, error) {
	if m.job != nil && m.job.ID == id {
		if status, ok := updates["status"].(string); ok {
			m.job.Status = status
		}
		return m.job, nil
	}
	return nil, errors.New("job not found")
}

func (m *testWorkflowRepo) CreateCheckpoint(ctx context.Context, cp models.WorkflowCheckpoint) error {
	m.checkpoints = append(m.checkpoints, cp)
	return nil
}

func (m *testWorkflowRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	return m.checkpoints, nil
}

func (m *testWorkflowRepo) DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error {
	var remaining []models.WorkflowCheckpoint
	for _, cp := range m.checkpoints {
		match := false
		for _, step := range steps {
			if cp.Step == step {
				match = true
				break
			}
		}
		if !match {
			remaining = append(remaining, cp)
		}
	}
	m.checkpoints = remaining
	return nil
}

func (m *testWorkflowRepo) ResetStuckJobs(ctx context.Context) error {
	return nil
}

func (m *testWorkflowRepo) CreateLog(ctx context.Context, log models.TaskLog) error {
	return nil
}

func (m *testWorkflowRepo) ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	return nil, nil
}

func (m *testWorkflowRepo) AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error) {
	return "mock-conn", true, nil
}

func (m *testWorkflowRepo) ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error {
	return nil
}

func (m *testWorkflowRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	return nil
}

// Mock TaskService
type mockTaskSvc struct {
	TaskService
	task *models.Task
}

func (m *mockTaskSvc) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return m.task, nil
}

func (m *mockTaskSvc) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	if input.Status != nil {
		m.task.Status = *input.Status
	}
	return m.task, nil
}

// Mock AuditService
type mockAuditSvc struct {
	AuditService
	recordedActions []string
}

func (m *mockAuditSvc) RecordAction(ctx context.Context, action, resource, resourceID string, opts ...service.AuditOption) {
	m.recordedActions = append(m.recordedActions, action)
}

func TestPRHandler_Reject_TriggersRepair(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Status:     models.TaskStatusHumanReview,
		SpecStatus: models.TaskSpecStatusApproved,
	}

	taskRepo := &testTaskRepo{task: task}
	workflowRepo := &testWorkflowRepo{}

	// Initialize orchestrator with mocks
	orch := orchestrator.New(taskRepo, workflowRepo, nil, nil)

	taskSvc := &mockTaskSvc{task: task}
	auditSvc := &mockAuditSvc{}

	handler := NewPRHandler(taskSvc, auditSvc, orch)

	r := chi.NewRouter()
	r.Post("/api/v1/tasks/{taskID}/pr/reject", handler.Reject)

	// Pre-seed some checkpoints
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-123", Step: "analyze"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-123", Step: "plan"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-123", Step: "review"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-123", Step: "fix"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-123", Step: "test"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-123", Step: "pr"})

	// Reject PR payload
	body := `{"feedback": "Please fix formatting and build failures."}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/tasks/task-123/pr/reject", strings.NewReader(body))
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// 1. Verify task status was updated to fixing
	if task.Status != models.TaskStatusFixing {
		t.Errorf("expected task status to be fixing, got %s", task.Status)
	}

	// 2. Verify audit action recorded
	foundRejectAction := false
	for _, act := range auditSvc.recordedActions {
		if act == models.AuditActionPRRejected {
			foundRejectAction = true
		}
	}
	if !foundRejectAction {
		t.Errorf("expected audit action %s, got %v", models.AuditActionPRRejected, auditSvc.recordedActions)
	}

	// 3. Verify checkpoints for review, fix, test, and pr were deleted
	cps, _ := workflowRepo.ListCheckpoints(context.Background(), "task-123")
	expectedSteps := map[string]bool{
		"analyze":      true,
		"plan":         true,
		"pr_rejection": true, // saved rejection feedback
		"review":       false,
		"fix":          false,
		"test":         false,
		"pr":           false,
	}

	for _, cp := range cps {
		if val, exists := expectedSteps[cp.Step]; exists {
			if !val {
				t.Errorf("checkpoint for step %s should have been deleted, but exists", cp.Step)
			}
		}
	}

	// 4. Verify workflow job was enqueued
	if workflowRepo.job == nil {
		t.Error("expected workflow job to be enqueued/started, but got nil")
	} else if workflowRepo.job.Status != models.WorkflowJobStatusQueued {
		t.Errorf("expected workflow job status queued, got %s", workflowRepo.job.Status)
	}
}

func TestPRHandler_StartReview_And_Approve(t *testing.T) {
	task := &models.Task{
		ID:         "task-456",
		Status:     models.TaskStatusPrReady,
		SpecStatus: models.TaskSpecStatusApproved,
	}

	taskRepo := &testTaskRepo{task: task}
	workflowRepo := &testWorkflowRepo{}
	orch := orchestrator.New(taskRepo, workflowRepo, nil, nil)
	taskSvc := &mockTaskSvc{task: task}
	auditSvc := &mockAuditSvc{}

	handler := NewPRHandler(taskSvc, auditSvc, orch)

	r := chi.NewRouter()
	r.Post("/api/v1/tasks/{taskID}/pr/start-review", handler.StartReview)
	r.Post("/api/v1/tasks/{taskID}/pr/approve", handler.Approve)

	// 1. Verify we can start review from pr_ready
	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/api/v1/tasks/task-456/pr/start-review", nil)
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr1.Code, rr1.Body.String())
	}
	if task.Status != models.TaskStatusHumanReview {
		t.Errorf("expected status human_review after start-review, got %s", task.Status)
	}

	// 2. Verify we can approve from human_review
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/tasks/task-456/pr/approve", nil)
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr2.Code, rr2.Body.String())
	}
	if task.Status != models.TaskStatusMerged {
		t.Errorf("expected status merged after approve, got %s", task.Status)
	}
}

func TestPRHandler_Reject_ExceedsReviewLoopLimit(t *testing.T) {
	task := &models.Task{
		ID:         "task-789",
		ProjectID:  "proj-123",
		Status:     models.TaskStatusHumanReview,
		SpecStatus: models.TaskSpecStatusApproved,
	}

	taskRepo := &testTaskRepo{task: task}
	workflowRepo := &testWorkflowRepo{}

	// Seed 3 pr_rejection checkpoints to hit default limit of 3
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-789", Step: "pr_rejection"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-789", Step: "pr_rejection"})
	_ = workflowRepo.CreateCheckpoint(context.Background(), models.WorkflowCheckpoint{TaskID: "task-789", Step: "pr_rejection"})

	orch := orchestrator.New(taskRepo, workflowRepo, nil, nil)
	taskSvc := &mockTaskSvc{task: task}
	auditSvc := &mockAuditSvc{}

	handler := NewPRHandler(taskSvc, auditSvc, orch)

	r := chi.NewRouter()
	r.Post("/api/v1/tasks/{taskID}/pr/reject", handler.Reject)

	body := `{"feedback": "Please fix formatting."}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/tasks/task-789/pr/reject", strings.NewReader(body))
	r.ServeHTTP(rr, req)

	// Since limit is 3 and we have 3 rejection checkpoints, it should fail
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	if task.Status != models.TaskStatusFailed {
		t.Errorf("expected task status to be updated to failed, got %s", task.Status)
	}
}
