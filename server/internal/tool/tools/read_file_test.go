package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestReadFileTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file with 10 lines
	contentLines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"line 9",
		"line 10",
	}
	testFilePath := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFilePath, []byte(strings.Join(contentLines, "\n")), 0o644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	rft := &ReadFileTool{}

	// Test 1: Full file read with max_lines truncation (set max_lines to 5)
	res, err := rft.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"path": "test.txt", "max_lines": 5},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute tool: %v", err)
	}
	if !res.Success {
		t.Fatalf("tool execution failed: %s", res.Message)
	}
	expectedOutput := "line 1\nline 2\nline 3\nline 4\nline 5"
	if res.Output != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, res.Output)
	}
	retLines, _ := res.Metadata["returned_lines"].(int)
	if retLines != 5 {
		t.Errorf("expected returned_lines to be 5, got %v", retLines)
	}

	// Test 2: Line range read (start_line=3, end_line=6)
	resRange, err := rft.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"path": "test.txt", "start_line": 3, "end_line": 6},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute tool: %v", err)
	}
	if !resRange.Success {
		t.Fatalf("tool execution failed: %s", resRange.Message)
	}
	expectedRange := "line 3\nline 4\nline 5\nline 6"
	if resRange.Output != expectedRange {
		t.Errorf("expected range output %q, got %q", expectedRange, resRange.Output)
	}
	startLine, _ := resRange.Metadata["start_line"].(int)
	endLine, _ := resRange.Metadata["end_line"].(int)
	if startLine != 3 || endLine != 6 {
		t.Errorf("expected metadata range 3-6, got %v-%v", startLine, endLine)
	}

	// Test 3: Around line read (around_line=5, radius=2) -> lines 3-7
	resAround, err := rft.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"path": "test.txt", "around_line": 5, "radius": 2},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute tool: %v", err)
	}
	if !resAround.Success {
		t.Fatalf("tool execution failed: %s", resAround.Message)
	}
	expectedAround := "line 3\nline 4\nline 5\nline 6\nline 7"
	if resAround.Output != expectedAround {
		t.Errorf("expected around output %q, got %q", expectedAround, resAround.Output)
	}

	// Test 4: Path traversal rejection
	resTraversal, err := rft.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"path": "../outside.txt"},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute tool: %v", err)
	}
	if resTraversal.Success {
		t.Errorf("expected path traversal to fail")
	}
	if len(resTraversal.Diagnostics) != 1 || resTraversal.Diagnostics[0].Severity != "error" {
		t.Errorf("expected error diagnostic on path traversal failure")
	}
}
