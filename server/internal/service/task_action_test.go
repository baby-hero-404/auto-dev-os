package service

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type fakeOrch struct {
	executeCalls int
	pauseCalls   int
}

func (f *fakeOrch) Execute(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	f.executeCalls++
	return &models.WorkflowJob{}, nil
}
func (f *fakeOrch) PauseJob(ctx context.Context, taskID string) error {
	f.pauseCalls++
	return nil
}
func (f *fakeOrch) CancelJob(ctx context.Context, taskID string) error { return nil }
func (f *fakeOrch) RetryFromLastStep(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	return &models.WorkflowJob{}, nil
}
func (f *fakeOrch) RetryBlockedParent(ctx context.Context, parentID string) (*models.Task, error) {
	return &models.Task{ID: parentID}, nil
}
func (f *fakeOrch) CheckSpecReviewLoopLimit(ctx context.Context, taskID string) error { return nil }
func (f *fakeOrch) SaveSpecReviewCycle(ctx context.Context, taskID, comment string) error {
	return nil
}

func newTaskActionServiceTestDB(t *testing.T) (*TaskActionService, *fakeOrch, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		db.Close()
		t.Fatalf("failed to open gorm db: %v", err)
	}
	taskSvc := NewTaskService(repository.NewTaskRepo(gormDB), repository.NewProjectRepo(gormDB), nil, repository.NewOrganizationRepo(gormDB))
	orch := &fakeOrch{}
	actionSvc := NewTaskActionService(taskSvc, repository.NewTaskActionRequestRepo(gormDB), orch)
	return actionSvc, orch, mock, func() { _ = db.Close() }
}

func expectFindActionRequestNotFound(mock sqlmock.Sqlmock, taskID, requestID string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "task_action_requests" WHERE task_id = $1 AND request_id = $2 ORDER BY "task_action_requests"."id" LIMIT $3`)).
		WithArgs(taskID, requestID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
}

func expectTaskGetByID(mock sqlmock.Sqlmock, taskID, status string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tasks" WHERE id = $1 ORDER BY "tasks"."id" LIMIT $2`)).
		WithArgs(taskID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(taskID, status))
}

// TestDispatch_InsufficientRole_NoStateChange asserts a caller without the
// required role for "delete" is rejected before any task state is touched
// (tasks.md 2.4 Verify: "caller without project.write posting pause -> 403,
// no state change" — delete/admin is this codebase's equivalent gated
// action since it has no project.write/admin permission granularity, see
// task_action.go's actionRolePolicy).
func TestDispatch_InsufficientRole_NoStateChange(t *testing.T) {
	svc, orch, mock, cleanup := newTaskActionServiceTestDB(t)
	defer cleanup()

	expectFindActionRequestNotFound(mock, "task-1", "req-1")

	_, err := svc.Dispatch(context.Background(), "task-1", models.UserRoleMember, TaskActionInput{
		Action: "delete", RequestID: "req-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
	if orch.executeCalls != 0 || orch.pauseCalls != 0 {
		t.Fatalf("expected no orchestrator side effects, got execute=%d pause=%d", orch.executeCalls, orch.pauseCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB call (task state should never be reached): %v", err)
	}
}

// TestDispatch_ActionNoLongerAvailable_Conflict asserts an action valid at
// GET time but no longer in the task's current available_actions (task
// moved on) is rejected with a conflict rather than executed.
func TestDispatch_ActionNoLongerAvailable_Conflict(t *testing.T) {
	svc, orch, mock, cleanup := newTaskActionServiceTestDB(t)
	defer cleanup()

	expectFindActionRequestNotFound(mock, "task-1", "req-1")
	expectTaskGetByID(mock, "task-1", models.TaskStatusMerged) // merged -> zero available actions

	_, err := svc.Dispatch(context.Background(), "task-1", models.UserRoleAdmin, TaskActionInput{
		Action: "pause", RequestID: "req-1",
	})
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if orch.executeCalls != 0 || orch.pauseCalls != 0 {
		t.Fatalf("expected no orchestrator side effects, got execute=%d pause=%d", orch.executeCalls, orch.pauseCalls)
	}
}

// TestDispatch_IdempotentDoubleSubmit asserts a repeated (task_id,
// request_id) returns the stored response without re-executing the action.
func TestDispatch_IdempotentDoubleSubmit(t *testing.T) {
	svc, _, mock, cleanup := newTaskActionServiceTestDB(t)
	defer cleanup()

	stored := []byte(`{"id":"task-1","status":"coding"}`)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "task_action_requests" WHERE task_id = $1 AND request_id = $2 ORDER BY "task_action_requests"."id" LIMIT $3`)).
		WithArgs("task-1", "req-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "request_id", "action", "response"}).
			AddRow("row-1", "task-1", "req-1", "approve_spec", stored))

	got, err := svc.Dispatch(context.Background(), "task-1", models.UserRoleMember, TaskActionInput{
		Action: "approve_spec", RequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "coding" {
		t.Fatalf("expected stored response to be returned, got status %q", got.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected only the idempotency lookup query, got: %v", err)
	}
}
