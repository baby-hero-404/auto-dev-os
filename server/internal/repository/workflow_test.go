package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
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
