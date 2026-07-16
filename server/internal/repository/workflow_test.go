package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestWorkflowRepoFileLogging(t *testing.T) {
	tempDir := t.TempDir()

	repo := NewWorkflowRepo(nil)
	repo.SetLogFileRoot(tempDir)

	ctx := context.Background()
	taskID := "task-123-abc"

	// 1. Log list should be empty initially
	logs, err := repo.ListLogs(ctx, taskID)
	if err != nil {
		t.Fatalf("ListLogs failed on empty directory: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}

	// 2. Create logs
	log1 := models.TaskLog{
		TaskID:    taskID,
		Level:     "info",
		Message:   "First log line",
		CreatedAt: time.Now().Add(-10 * time.Minute),
	}
	log2 := models.TaskLog{
		TaskID:    taskID,
		Level:     "error",
		Message:   "Second log line (error)",
		CreatedAt: time.Now(),
	}

	if err := repo.CreateLog(ctx, log1); err != nil {
		t.Fatalf("CreateLog 1 failed: %v", err)
	}
	if err := repo.CreateLog(ctx, log2); err != nil {
		t.Fatalf("CreateLog 2 failed: %v", err)
	}

	// 3. List logs and assert properties
	logs, err = repo.ListLogs(ctx, taskID)
	if err != nil {
		t.Fatalf("ListLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}

	if logs[0].Message != "First log line" || logs[0].Level != "info" {
		t.Errorf("unexpected first log: %+v", logs[0])
	}
	if logs[1].Message != "Second log line (error)" || logs[1].Level != "error" {
		t.Errorf("unexpected second log: %+v", logs[1])
	}

	// 4. Verify file was created in correct path
	expectedFile := filepath.Join(tempDir, taskID+".jsonl")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected log file at %s to exist, but got error: %v", expectedFile, err)
	}
}

func TestWorkflowRepo_TailLogs(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewWorkflowRepo(nil)
	repo.SetLogFileRoot(tempDir)
	ctx := context.Background()
	taskID := "task-tail"

	// 1. Empty file
	logs, err := repo.TailLogs(ctx, taskID, 500)
	if err != nil {
		t.Fatalf("TailLogs on empty/missing file failed: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}

	// 2. < N lines
	for i := 0; i < 5; i++ {
		_ = repo.CreateLog(ctx, models.TaskLog{TaskID: taskID, Message: fmt.Sprintf("Log %d", i)})
	}
	logs, err = repo.TailLogs(ctx, taskID, 10)
	if err != nil {
		t.Fatalf("TailLogs failed: %v", err)
	}
	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}

	// 3. > N lines
	for i := 5; i < 15; i++ {
		_ = repo.CreateLog(ctx, models.TaskLog{TaskID: taskID, Message: fmt.Sprintf("Log %d", i)})
	}
	logs, err = repo.TailLogs(ctx, taskID, 10)
	if err != nil {
		t.Fatalf("TailLogs failed: %v", err)
	}
	if len(logs) != 10 {
		t.Errorf("expected 10 logs, got %d", len(logs))
	}
	if logs[0].Message != "Log 5" || logs[9].Message != "Log 14" {
		t.Errorf("expected logs 5 to 14, got %s to %s", logs[0].Message, logs[9].Message)
	}
}

func TestWorkflowRepo_LogHubConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewWorkflowRepo(nil)
	repo.SetLogFileRoot(tempDir)
	taskID := "task-hub"
	
	// Start 10 subscribers
	var subs []chan models.TaskLog
	for i := 0; i < 10; i++ {
		subs = append(subs, repo.SubscribeLogs(taskID))
	}

	// Broadcast 100 messages concurrently
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = repo.CreateLog(context.Background(), models.TaskLog{TaskID: taskID, Message: fmt.Sprintf("msg %d", idx)})
		}(i)
	}
	wg.Wait()

	// Each subscriber should have received 100 messages
	for i, sub := range subs {
		if len(sub) != 100 {
			t.Errorf("subscriber %d received %d messages, expected 100", i, len(sub))
		}
		repo.UnsubscribeLogs(taskID, sub)
	}

	// Unsubscribe all, check map cleanup
	repo.LogHub.mu.Lock()
	if len(repo.LogHub.subscribers[taskID]) != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", len(repo.LogHub.subscribers[taskID]))
	}
	repo.LogHub.mu.Unlock()
}

// seedGateFixtureLogs writes the same log shape (2 "assembled prompt with..." info
// calls, 1 "[TELEMETRY-VIOLATION]..." warn call, 1 unrelated info call) for taskID via
// repo.CreateLog, so file-mode and DB-mode tests exercise identical fixture data.
func seedGateFixtureLogs(t *testing.T, repo *WorkflowRepo, taskID string) {
	t.Helper()
	ctx := context.Background()
	logs := []models.TaskLog{
		{TaskID: taskID, Level: "info", Message: "assembled prompt with 2 messages and 7 tools"},
		{TaskID: taskID, Level: "info", Message: "assembled prompt with 3 messages and 5 tools"},
		{TaskID: taskID, Level: "warn", Message: "[TELEMETRY-VIOLATION] Shadow state machine: tool run_build is not permitted during state IMPLEMENTATION"},
		{TaskID: taskID, Level: "info", Message: "step context_load running"},
	}
	for _, l := range logs {
		if err := repo.CreateLog(ctx, l); err != nil {
			t.Fatalf("seed CreateLog failed: %v", err)
		}
	}
}

func TestWorkflowRepo_FindLogsByPattern_FileMode(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewWorkflowRepo(nil)
	repo.SetLogFileRoot(tempDir)
	ctx := context.Background()
	taskID := "task-pattern-file"
	seedGateFixtureLogs(t, repo, taskID)

	violations, err := repo.FindLogsByPattern(ctx, []string{taskID}, "warn", "[TELEMETRY-VIOLATION]")
	if err != nil {
		t.Fatalf("FindLogsByPattern failed: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Message, "TELEMETRY-VIOLATION") {
		t.Errorf("unexpected matched message: %q", violations[0].Message)
	}

	calls, err := repo.FindLogsByPattern(ctx, []string{taskID}, "info", "assembled prompt with")
	if err != nil {
		t.Fatalf("FindLogsByPattern failed: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}

	// A task with no log file at all must not error — only tasks that ran produce a file.
	none, err := repo.FindLogsByPattern(ctx, []string{"task-never-ran"}, "warn", "[TELEMETRY-VIOLATION]")
	if err != nil {
		t.Fatalf("FindLogsByPattern on missing file should not error: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 matches for missing task, got %d", len(none))
	}

	// Empty taskIDs short-circuits without touching the filesystem.
	empty, err := repo.FindLogsByPattern(ctx, nil, "warn", "[TELEMETRY-VIOLATION]")
	if err != nil || len(empty) != 0 {
		t.Errorf("expected (nil, nil) for empty taskIDs, got (%v, %v)", empty, err)
	}
}

func TestWorkflowRepo_CountTotalCalls_FileMode(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewWorkflowRepo(nil)
	repo.SetLogFileRoot(tempDir)
	ctx := context.Background()
	taskA, taskB := "task-count-a", "task-count-b"
	seedGateFixtureLogs(t, repo, taskA)
	seedGateFixtureLogs(t, repo, taskB)

	count, err := repo.CountTotalCalls(ctx, []string{taskA, taskB})
	if err != nil {
		t.Fatalf("CountTotalCalls failed: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 total calls across 2 tasks (2 each), got %d", count)
	}

	zero, err := repo.CountTotalCalls(ctx, nil)
	if err != nil || zero != 0 {
		t.Errorf("expected (0, nil) for empty taskIDs, got (%d, %v)", zero, err)
	}
}

func TestWorkflowRepo_FindLogsByPattern_DBMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}
	repo := NewWorkflowRepo(gormDB) // fileRoot=="" by default: DB mode

	rows := sqlmock.NewRows([]string{"id", "task_id", "job_id", "level", "message", "created_at"}).
		AddRow("log-1", "task-db-1", nil, "warn", "[TELEMETRY-VIOLATION] tool run_build is not permitted", time.Now())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "task_logs" WHERE task_id IN ($1) AND level = $2 AND message LIKE $3`)).
		WithArgs("task-db-1", "warn", "[TELEMETRY-VIOLATION]%").
		WillReturnRows(rows)

	violations, err := repo.FindLogsByPattern(context.Background(), []string{"task-db-1"}, "warn", "[TELEMETRY-VIOLATION]")
	if err != nil {
		t.Fatalf("FindLogsByPattern (DB mode) failed: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestWorkflowRepo_CountTotalCalls_DBMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}
	repo := NewWorkflowRepo(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "task_logs" WHERE task_id IN ($1) AND message LIKE 'assembled prompt with%'`)).
		WithArgs("task-db-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := repo.CountTotalCalls(context.Background(), []string{"task-db-1"})
	if err != nil {
		t.Fatalf("CountTotalCalls (DB mode) failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
