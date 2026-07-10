package verify

import (
	"context"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

type mockRuntime struct {
	run func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error)
}

func (m *mockRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	return m.run(ctx, req)
}

func (m *mockRuntime) Prewarm(ctx context.Context) error { return nil }

func TestGofmtHook(t *testing.T) {
	calledGofmt := false
	mr := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			cmdStr := strings.Join(req.Command, " ")
			if strings.HasPrefix(cmdStr, "gofmt -w") {
				calledGofmt = true
				return &sandbox.CommandResult{ExitCode: 0}, nil
			}
			return &sandbox.CommandResult{ExitCode: 0}, nil
		},
	}

	hook := NewGofmtHook(mr)
	diags := hook.Run(context.Background(), "/tmp", []string{"main.go", "readme.md"})
	if len(diags) != 0 {
		t.Errorf("Expected 0 diagnostics, got %v", diags)
	}
	if !calledGofmt {
		t.Errorf("Expected gofmt -w to be run on main.go")
	}
}

func TestCompileCheckHook(t *testing.T) {
	mr := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			return &sandbox.CommandResult{
				ExitCode: 2,
				Stderr:   "main.go:10: syntax error: unexpected func",
			}, nil
		},
	}

	hook := NewCompileCheckHook(mr)
	diags := hook.Run(context.Background(), "/tmp", []string{"main.go"})
	if len(diags) != 1 {
		t.Fatalf("Expected 1 diagnostic, got %d", len(diags))
	}
	d := diags[0]
	if d.Severity != "error" || d.File != "main.go" || d.Line != 10 || !strings.Contains(d.Message, "syntax error") {
		t.Errorf("Diagnostic mismatch: %+v", d)
	}
}

func TestVerifyPipeline(t *testing.T) {
	mrFail := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			return &sandbox.CommandResult{
				ExitCode: 2,
				Stderr:   "main.go:10: syntax error: unexpected func",
			}, nil
		},
	}
	mrPass := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			return &sandbox.CommandResult{ExitCode: 0}, nil
		},
	}

	gofmt := NewGofmtHook(mrPass)
	compile := NewCompileCheckHook(mrFail)

	pipeline := &tool.VerifyPipeline{
		Hooks: []tool.VerifyHook{gofmt, compile},
	}

	diags := pipeline.Run(context.Background(), "/tmp", []string{"main.go"})
	if len(diags) != 1 {
		t.Fatalf("Expected 1 diagnostic from compile check, got %d", len(diags))
	}
	if diags[0].Message != "syntax error: unexpected func" {
		t.Errorf("Expected syntax error diagnostic, got %+v", diags[0])
	}
}
