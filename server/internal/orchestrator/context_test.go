package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestFileContextRetriever_Retrieve(t *testing.T) {
	// Create temporary mock repository directory structure.
	tempDir, err := os.MkdirTemp("", "mock-repo")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some files.
	file1Path := filepath.Join(tempDir, "file1.go")
	file1Content := `package main

import "pkg/helper"

func main() {}
`
	if err := os.WriteFile(file1Path, []byte(file1Content), 0o644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	// Create package directory.
	pkgDir := filepath.Join(tempDir, "pkg", "helper")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}
	helperPath := filepath.Join(pkgDir, "helper.go")
	helperContent := `package helper
func Help() {}
`
	if err := os.WriteFile(helperPath, []byte(helperContent), 0o644); err != nil {
		t.Fatalf("failed to write helper: %v", err)
	}

	retriever := NewFileContextRetriever()

	// Test case 1: Explicit files.
	analysisJSON, _ := json.Marshal(map[string]any{
		"affected_files": []string{"file1.go"},
	})
	task := models.Task{
		Title:       "Test task",
		Description: "A test description",
		Analysis:    analysisJSON,
	}

	files, err := retriever.Retrieve(context.Background(), task, tempDir)
	if err != nil {
		t.Fatalf("expected context files, got error: %v", err)
	}

	// We expect file1.go and helper.go (via import scanner)
	foundFile1 := false
	foundHelper := false
	for _, f := range files {
		if f.Path == "file1.go" {
			foundFile1 = true
		}
		if filepath.ToSlash(f.Path) == "pkg/helper/helper.go" {
			foundHelper = true
		}
	}

	if !foundFile1 {
		t.Error("expected to retrieve file1.go")
	}
	if !foundHelper {
		t.Error("expected to retrieve pkg/helper/helper.go via import scanner")
	}

	// Test case 2: Keyword fallback.
	taskNoAnalysis := models.Task{
		Title:       "Update helper logic",
		Description: "Refactor the helper methods",
	}

	filesFallback, err := retriever.Retrieve(context.Background(), taskNoAnalysis, tempDir)
	if err != nil {
		t.Fatalf("expected context files on fallback, got error: %v", err)
	}

	foundHelperFallback := false
	for _, f := range filesFallback {
		if filepath.ToSlash(f.Path) == "pkg/helper/helper.go" {
			foundHelperFallback = true
		}
	}

	if !foundHelperFallback {
		t.Error("expected to retrieve pkg/helper/helper.go via keyword fallback")
	}
}
