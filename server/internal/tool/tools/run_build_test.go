package tools

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestRunBuildTool(t *testing.T) {
	compilerOutput := `
# github.com/auto-code-os/auto-code-os/server/internal/tool
internal/tool/registry.go:12:3: undefined: toolInterface
internal/tool/tool.go:45: syntax error: unexpected newline
`
	mr := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			return &sandbox.CommandResult{
				ExitCode: 2,
				Stdout:   "",
				Stderr:   compilerOutput,
			}, nil
		},
	}

	rbt := NewRunBuildTool(mr)

	res, err := rbt.Execute(context.Background(), tool.Call{Workspace: "/tmp"})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if res.Success {
		t.Errorf("expected build to fail")
	}

	diags := res.Diagnostics
	if len(diags) != 2 {
		t.Fatalf("expected 2 error diagnostics, got %d", len(diags))
	}

	d1 := diags[0]
	if d1.File != "internal/tool/registry.go" || d1.Line != 12 || d1.Message != "undefined: toolInterface" {
		t.Errorf("d1 mismatch: %+v", d1)
	}

	d2 := diags[1]
	if d2.File != "internal/tool/tool.go" || d2.Line != 45 || d2.Message != "syntax error: unexpected newline" {
		t.Errorf("d2 mismatch: %+v", d2)
	}
}
