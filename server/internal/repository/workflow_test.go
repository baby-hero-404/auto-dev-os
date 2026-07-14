package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
