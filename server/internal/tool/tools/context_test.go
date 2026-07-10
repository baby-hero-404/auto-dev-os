package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

type mockAffectedFilesProvider struct {
	files []string
}

func (m *mockAffectedFilesProvider) GetAffectedFiles(ctx context.Context, taskID string) ([]string, error) {
	return m.files, nil
}

func TestReadSpecAndAffectedFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock spec directory structure
	specDir := filepath.Join(tmpDir, "docs", "openspecs", "my-task")
	err = os.MkdirAll(specDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create spec dir: %v", err)
	}

	err = os.WriteFile(filepath.Join(specDir, "specs.md"), []byte("# Spec Title\nRequirement 1"), 0o644)
	if err != nil {
		t.Fatalf("failed to write specs: %v", err)
	}
	err = os.WriteFile(filepath.Join(specDir, "design.md"), []byte("# Design\nImplementation Details"), 0o644)
	if err != nil {
		t.Fatalf("failed to write design: %v", err)
	}

	// Create mock affected files in workspace
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644)
	if err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package util"), 0o644)
	if err != nil {
		t.Fatalf("failed to write util.go: %v", err)
	}

	// Test 1: ReadSpecTool
	rst := &ReadSpecTool{}
	resSpec, err := rst.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"task": "my-task"},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute read_spec: %v", err)
	}
	if !resSpec.Success {
		t.Fatalf("expected read_spec success, got %v", resSpec.Message)
	}
	specOutput := resSpec.Output
	if !strings.Contains(specOutput, "specs.md") || !strings.Contains(specOutput, "design.md") {
		t.Errorf("expected spec output to contain both files, got %v", specOutput)
	}
	if !strings.Contains(specOutput, "Requirement 1") || !strings.Contains(specOutput, "Implementation Details") {
		t.Errorf("expected spec content to be present, got %v", specOutput)
	}

	// Test 2: ReadAffectedFilesTool
	provider := &mockAffectedFilesProvider{
		files: []string{"main.go", "util.go"},
	}
	raft := NewReadAffectedFilesTool(provider)
	resAffected, err := raft.Execute(context.Background(), tool.Call{
		TaskID:    "my-task-id",
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute read_affected_files: %v", err)
	}
	if !resAffected.Success {
		t.Fatalf("expected read_affected_files success, got %v", resAffected.Message)
	}
	affectedOutput := resAffected.Output
	if !strings.Contains(affectedOutput, "main.go") || !strings.Contains(affectedOutput, "util.go") {
		t.Errorf("expected affected output to contain main.go and util.go, got %v", affectedOutput)
	}
	readFiles, _ := resAffected.Metadata["files_read"].([]string)
	if len(readFiles) != 2 || readFiles[0] != "main.go" || readFiles[1] != "util.go" {
		t.Errorf("expected read files metadata to be [main.go, util.go], got %v", readFiles)
	}
}
