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
	expectedOutput := "1\tline 1\n2\tline 2\n3\tline 3\n4\tline 4\n5\tline 5"
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
	expectedRange := "3\tline 3\n4\tline 4\n5\tline 5\n6\tline 6"
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
	expectedAround := "3\tline 3\n4\tline 4\n5\tline 5\n6\tline 6\n7\tline 7"
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

	// Test 5: Batch read via "paths" (multiple files in one call)
	testFilePath2 := filepath.Join(tmpDir, "second.txt")
	if err := os.WriteFile(testFilePath2, []byte("alpha\nbeta"), 0o644); err != nil {
		t.Fatalf("failed to write second test file: %v", err)
	}
	resBatch, err := rft.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"paths": []string{"test.txt", "second.txt"}, "start_line": 1, "end_line": 2},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute tool: %v", err)
	}
	if !resBatch.Success {
		t.Fatalf("batch tool execution failed: %s", resBatch.Message)
	}
	if !strings.Contains(resBatch.Output, "### test.txt") || !strings.Contains(resBatch.Output, "### second.txt") {
		t.Errorf("expected batch output to contain per-file headers, got %q", resBatch.Output)
	}
	if !strings.Contains(resBatch.Output, "1\talpha\n2\tbeta") {
		t.Errorf("expected batch output to contain numbered content for second.txt, got %q", resBatch.Output)
	}
	files, _ := resBatch.Metadata["files"].([]map[string]any)
	if len(files) != 2 {
		t.Errorf("expected files metadata for 2 files, got %d", len(files))
	}

	// Test 6: Batch read where one path fails should still return partial success
	resBatchPartial, err := rft.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"paths": []string{"test.txt", "missing.txt"}},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute tool: %v", err)
	}
	if !resBatchPartial.Success {
		t.Errorf("expected partial batch success when at least one file reads successfully")
	}
	if len(resBatchPartial.Diagnostics) != 1 {
		t.Errorf("expected exactly one diagnostic for the missing file, got %d", len(resBatchPartial.Diagnostics))
	}
}
