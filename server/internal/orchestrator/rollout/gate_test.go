package rollout

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newFixtureTaskRepo returns a TaskRepo backed by sqlmock that returns the given
// task IDs from ListRecentByStatus, regardless of the requested statuses/limit —
// the gate only cares about the IDs it gets back to seed its log lookups.
func newFixtureTaskRepo(t *testing.T, taskIDs []string) *repository.TaskRepo {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	rows := sqlmock.NewRows([]string{"id"})
	for _, id := range taskIDs {
		rows.AddRow(id)
	}
	mock.MatchExpectationsInOrder(false)
	mock.ExpectQuery(`SELECT \* FROM "tasks" WHERE status IN`).WillReturnRows(rows)

	return repository.NewTaskRepo(gormDB)
}

// newFixtureWorkflowRepo returns a file-mode WorkflowRepo and seeds nCalls
// "assembled prompt with..." info logs and nViolations "[TELEMETRY-VIOLATION]..."
// warn logs for taskID.
func newFixtureWorkflowRepo(t *testing.T, taskID string, nCalls, nViolations int) *repository.WorkflowRepo {
	t.Helper()
	repo := repository.NewWorkflowRepo(nil)
	repo.SetLogFileRoot(t.TempDir())
	ctx := context.Background()

	for i := 0; i < nCalls; i++ {
		if err := repo.CreateLog(ctx, models.TaskLog{TaskID: taskID, Level: "info", Message: "assembled prompt with 2 messages and 7 tools"}); err != nil {
			t.Fatalf("seed call log failed: %v", err)
		}
	}
	for i := 0; i < nViolations; i++ {
		if err := repo.CreateLog(ctx, models.TaskLog{TaskID: taskID, Level: "warn", Message: "[TELEMETRY-VIOLATION] Shadow state machine: tool run_build is not permitted during state IMPLEMENTATION"}); err != nil {
			t.Fatalf("seed violation log failed: %v", err)
		}
	}
	return repo
}

func TestEvaluateStateMachineGate_Pass(t *testing.T) {
	taskID := "task-pass"
	// 1 violation out of 100 calls = 1.0% — at (not over) a 2.0% threshold.
	tasks := newFixtureTaskRepo(t, []string{taskID})
	logs := newFixtureWorkflowRepo(t, taskID, 100, 1)

	result, err := EvaluateStateMachineGate(context.Background(), tasks, logs, 100, 2.0)
	if err != nil {
		t.Fatalf("EvaluateStateMachineGate failed: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true, got GateResult=%+v", result)
	}
	if result.TotalCalls != 100 {
		t.Errorf("expected TotalCalls=100, got %d", result.TotalCalls)
	}
	if result.TotalViolations != 1 {
		t.Errorf("expected TotalViolations=1, got %d", result.TotalViolations)
	}
	if result.ViolationRatePct != 1.0 {
		t.Errorf("expected ViolationRatePct=1.0, got %v", result.ViolationRatePct)
	}
}

func TestEvaluateStateMachineGate_Fail(t *testing.T) {
	taskID := "task-fail"
	// 10 violations out of 100 calls = 10% — over a 1.0% threshold.
	tasks := newFixtureTaskRepo(t, []string{taskID})
	logs := newFixtureWorkflowRepo(t, taskID, 100, 10)

	result, err := EvaluateStateMachineGate(context.Background(), tasks, logs, 100, 1.0)
	if err != nil {
		t.Fatalf("EvaluateStateMachineGate failed: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false, got GateResult=%+v", result)
	}
	if result.TotalViolations != 10 {
		t.Errorf("expected TotalViolations=10, got %d", result.TotalViolations)
	}
	if result.TopViolationTypes["tool not permitted"] != 10 {
		t.Errorf("expected 10 'tool not permitted' violations bucketed, got %+v", result.TopViolationTypes)
	}
}

func TestEvaluateStateMachineGate_ZeroCalls(t *testing.T) {
	taskID := "task-zero-calls"
	// Task sampled, but it produced no "assembled prompt with..." log lines at all
	// (e.g. failed before any LLM call) — must not divide by zero.
	tasks := newFixtureTaskRepo(t, []string{taskID})
	logs := newFixtureWorkflowRepo(t, taskID, 0, 0)

	result, err := EvaluateStateMachineGate(context.Background(), tasks, logs, 100, 1.0)
	if err != nil {
		t.Fatalf("EvaluateStateMachineGate failed: %v", err)
	}
	if result.TotalCalls != 0 {
		t.Errorf("expected TotalCalls=0, got %d", result.TotalCalls)
	}
	if result.ViolationRatePct != 0 {
		t.Errorf("expected ViolationRatePct=0 (no division-by-zero), got %v", result.ViolationRatePct)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true when there is nothing to violate, got GateResult=%+v", result)
	}
}

func TestEvaluateStateMachineGate_NoSampledTasks(t *testing.T) {
	// No terminal tasks exist yet at all — the gate must not fail or panic.
	tasks := newFixtureTaskRepo(t, nil)
	logs := newFixtureWorkflowRepo(t, "unused", 0, 0)

	result, err := EvaluateStateMachineGate(context.Background(), tasks, logs, 100, 1.0)
	if err != nil {
		t.Fatalf("EvaluateStateMachineGate failed: %v", err)
	}
	if result.TasksSampled != 0 {
		t.Errorf("expected TasksSampled=0, got %d", result.TasksSampled)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true when no tasks are sampled, got GateResult=%+v", result)
	}
}
