package tools

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestRunTestsTool(t *testing.T) {
	testOutput := `
=== RUN   TestReadFileTool
--- PASS: TestReadFileTool (0.01s)
=== RUN   TestSearchReplaceTool
--- FAIL: TestSearchReplaceTool (0.00s)
    search_replace_test.go:42: expected error
FAIL
`
	mr := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			return &sandbox.CommandResult{
				ExitCode: 1,
				Stdout:   testOutput,
				Stderr:   "",
			}, nil
		},
	}

	rtt := NewRunTestsTool(mr)

	res, err := rtt.Execute(context.Background(), tool.Call{Workspace: "/tmp"})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if res.Success {
		t.Errorf("expected success to be false due to exit code 1")
	}

	diags := res.Diagnostics
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics (1 pass, 1 fail), got %d", len(diags))
	}

	passFound := false
	failFound := false
	for _, d := range diags {
		if d.Severity == "info" && d.Message == "Test TestReadFileTool passed" {
			passFound = true
		}
		if d.Severity == "error" && d.Message == "Test TestSearchReplaceTool failed" {
			failFound = true
		}
	}

	if !passFound {
		t.Errorf("failed to find info diagnostic for passing test")
	}
	if !failFound {
		t.Errorf("failed to find error diagnostic for failing test")
	}
}
