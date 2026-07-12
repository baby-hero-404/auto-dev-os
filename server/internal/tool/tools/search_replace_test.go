package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

type mockVerifyHook struct {
	diags []tool.Diagnostic
}

func (m *mockVerifyHook) Name() string { return "mock" }
func (m *mockVerifyHook) Run(ctx context.Context, ws string, files []string) []tool.Diagnostic {
	return m.diags
}

func TestSearchReplaceTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sr-test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "code.go")
	initialContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	err = os.WriteFile(filePath, []byte(initialContent), 0o644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	srt := &SearchReplaceTool{}

	// Test 1: Dry run
	resDry, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "code.go",
			"search":  "println(\"hello\")",
			"replace": "println(\"world\")",
			"dry_run": true,
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resDry.Success {
		t.Errorf("expected success for dry run, got %v", resDry.Message)
	}
	preview, ok := resDry.Metadata["diff_preview"].(string)
	if !ok || !strings.Contains(preview, "println(\"world\")") {
		t.Errorf("expected preview to contain replaced string, got %v", preview)
	}
	// Verify file is unchanged
	data, _ := os.ReadFile(filePath)
	if string(data) != initialContent {
		t.Errorf("file was modified during dry run")
	}

	// Test 2: Successful search/replace
	resSuccess, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "code.go",
			"search":  "println(\"hello\")",
			"replace": "println(\"world\")",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resSuccess.Success {
		t.Errorf("expected success, got %v", resSuccess.Message)
	}
	data, _ = os.ReadFile(filePath)
	expectedContent := "package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n"
	if string(data) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(data))
	}
	if len(resSuccess.FilesChanged) != 1 || resSuccess.FilesChanged[0] != "code.go" {
		t.Errorf("FilesChanged mismatch")
	}

	// Test 3: Search block not found
	resNotFound, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "code.go",
			"search":  "println(\"nonexistent\")",
			"replace": "println(\"world\")",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if resNotFound.Success {
		t.Errorf("expected failure when search block is not found")
	}

	// Recreate initial state for ambiguity test
	ambiguousContent := "foo\nfoo\n"
	os.WriteFile(filePath, []byte(ambiguousContent), 0o644)

	// Test 4: Ambiguous match (2 occurrences)
	resAmbiguous, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "code.go",
			"search":  "foo",
			"replace": "bar",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if resAmbiguous.Success {
		t.Errorf("expected failure when search block is ambiguous")
	}
	data, _ = os.ReadFile(filePath)
	if string(data) != ambiguousContent {
		t.Errorf("file was modified during ambiguous run")
	}

	// Test 5: Verification failure triggers rollback
	os.WriteFile(filePath, []byte(initialContent), 0o644)
	mockHook := &mockVerifyHook{
		diags: []tool.Diagnostic{
			{Severity: "error", Message: "compilation failed"},
		},
	}
	pipeline := &tool.VerifyPipeline{Hooks: []tool.VerifyHook{mockHook}}
	srtWithVerify := &SearchReplaceTool{Verify: pipeline}

	resVerifyFail, err := srtWithVerify.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "code.go",
			"search":  "println(\"hello\")",
			"replace": "println(\"world\")",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if resVerifyFail.Success {
		t.Errorf("expected verification failure to return success: false")
	}
	// Verify rollback occurred
	data, _ = os.ReadFile(filePath)
	if string(data) != initialContent {
		t.Errorf("rollback failed, content is %q", string(data))
	}

	// Test 6: Rewrite entire file by passing search: ""
	resRewrite, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "code.go",
			"search":  "",
			"replace": "package main\n\nfunc main() {\n\tprintln(\"rewritten completely\")\n}\n",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute rewrite test: %v", err)
	}
	if !resRewrite.Success {
		t.Errorf("expected success for empty search rewrite, got %v", resRewrite.Message)
	}
	data, _ = os.ReadFile(filePath)
	expectedRewrite := "package main\n\nfunc main() {\n\tprintln(\"rewritten completely\")\n}\n"
	if string(data) != expectedRewrite {
		t.Errorf("expected rewritten content %q, got %q", expectedRewrite, string(data))
	}
}

func TestSearchReplaceTool_NonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sr-test-nonexistent")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srt := &SearchReplaceTool{}
	nonExistentPath := "nonexistent.go"

	// Case 1: Non-empty search string on non-existent file -> should return helpful diagnostic
	resErr, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    nonExistentPath,
			"search":  "println(\"hello\")",
			"replace": "println(\"world\")",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if resErr.Success {
		t.Errorf("expected failure for non-existent file with non-empty search")
	}
	if len(resErr.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics on failure")
	}
	msg := resErr.Diagnostics[0].Message
	if !strings.Contains(msg, "does not exist") || !strings.Contains(msg, "create_file") {
		t.Errorf("expected helpful diagnostic message, got %q", msg)
	}
	if strings.Contains(msg, "no such file or directory") {
		t.Errorf("raw OS error should not be leaked, got %q", msg)
	}

	// Case 2: Empty search string on non-existent file -> should create the file
	resCreate, err := srt.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    nonExistentPath,
			"search":  "",
			"replace": "package main\n",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !resCreate.Success {
		t.Errorf("expected success when creating a file with empty search, got: %s", resCreate.Message)
	}
	fullPath := filepath.Join(tmpDir, nonExistentPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("expected file to be created: %v", err)
	}
	if string(data) != "package main\n" {
		t.Errorf("expected file content to be 'package main\\n', got %q", string(data))
	}
}
