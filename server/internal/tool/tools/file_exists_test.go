package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestFileExistsTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "exists-test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFilePath := filepath.Join(tmpDir, "exists.txt")
	err = os.WriteFile(testFilePath, []byte("some content"), 0o644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fet := &FileExistsTool{}

	// Test 1: File exists
	resExists, err := fet.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"path": "exists.txt"},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resExists.Success {
		t.Errorf("expected success to be true, got %v", resExists.Message)
	}
	sizeVal, ok := resExists.Metadata["size"].(int64)
	if !ok {
		if sVal, ok := resExists.Metadata["size"].(int); ok {
			sizeVal = int64(sVal)
		}
	}
	if sizeVal != 12 {
		t.Errorf("expected size to be 12, got %v", sizeVal)
	}
	isDir, _ := resExists.Metadata["is_dir"].(bool)
	if isDir {
		t.Errorf("expected is_dir to be false")
	}

	// Test 2: File missing
	resMissing, err := fet.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"path": "missing.txt"},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resMissing.Success {
		t.Errorf("expected success to be true even when file is missing (to return structured result), got %v", resMissing.Message)
	}
	existsMissing, _ := resMissing.Metadata["exists"].(bool)
	if existsMissing {
		t.Errorf("expected exists metadata to be false for missing.txt")
	}
}
