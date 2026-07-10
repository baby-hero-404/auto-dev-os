package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestGrepSearchTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grep-test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	file1 := filepath.Join(tmpDir, "helper.go")
	content1 := "package utils\n\nfunc HelpMe() {\n\tprintln(\"helper logic\")\n}\n"
	os.WriteFile(file1, []byte(content1), 0o644)

	file2 := filepath.Join(tmpDir, "main.go")
	content2 := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tprintln(\"main logic\")\n}\n"
	os.WriteFile(file2, []byte(content2), 0o644)

	// Create vendor file (should be ignored)
	vendorDir := filepath.Join(tmpDir, "vendor")
	os.Mkdir(vendorDir, 0o755)
	file3 := filepath.Join(vendorDir, "library.go")
	content3 := "package vendor\n\nfunc Lib() {}\n"
	os.WriteFile(file3, []byte(content3), 0o644)

	gst := &GrepSearchTool{}

	// Test 1: Literal search returns line numbers
	resLiteral, err := gst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query": "println",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resLiteral.Success {
		t.Errorf("expected success, got %v", resLiteral.Message)
	}
	output := resLiteral.Output
	if !strings.Contains(output, "helper.go:4:\tprintln(\"helper logic\")") {
		t.Errorf("expected helper match, got %v", output)
	}
	if !strings.Contains(output, "main.go:6:\tprintln(\"main logic\")") {
		t.Errorf("expected main match, got %v", output)
	}
	if strings.Contains(output, "vendor") {
		t.Errorf("vendor files should have been skipped")
	}

	// Test 2: Regex search with capture/matching
	resRegex, err := gst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query": "func (H|m)[a-zA-Z]+",
			"regex": true,
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resRegex.Success {
		t.Errorf("expected success, got %v", resRegex.Message)
	}
	outputRegex := resRegex.Output
	if !strings.Contains(outputRegex, "helper.go:3:func HelpMe() {") {
		t.Errorf("expected func HelpMe match, got %v", outputRegex)
	}
	if !strings.Contains(outputRegex, "main.go:5:func main() {") {
		t.Errorf("expected func main match, got %v", outputRegex)
	}

	// Test 3: Include/exclude glob filtering
	// Include only helper.go
	resInclude, err := gst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query":   "println",
			"include": "*helper*",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resInclude.Success {
		t.Errorf("expected success, got %v", resInclude.Message)
	}
	outputInclude := resInclude.Output
	if !strings.Contains(outputInclude, "helper.go") {
		t.Errorf("expected helper.go match")
	}
	if strings.Contains(outputInclude, "main.go") {
		t.Errorf("main.go should be filtered out by include pattern")
	}

	// Exclude helper.go
	resExclude, err := gst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query":   "println",
			"exclude": "*helper*",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resExclude.Success {
		t.Errorf("expected success, got %v", resExclude.Message)
	}
	outputExclude := resExclude.Output
	if strings.Contains(outputExclude, "helper.go") {
		t.Errorf("helper.go should be filtered out by exclude pattern")
	}
	if !strings.Contains(outputExclude, "main.go") {
		t.Errorf("expected main.go match")
	}
}
