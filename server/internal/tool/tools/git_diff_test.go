package tools

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

func TestGitDiffTool(t *testing.T) {
	mr := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			cmdStr := strings.Join(req.Command, " ")
			if strings.Contains(cmdStr, "--name-only") {
				return &sandbox.CommandResult{
					ExitCode: 0,
					Stdout:   "main.go\ninternal/tool/tool.go\n",
				}, nil
			}
			return &sandbox.CommandResult{
				ExitCode: 0,
				Stdout:   "mock diff output",
			}, nil
		},
	}

	gdt := NewGitDiffTool(mr)

	res, err := gdt.Execute(context.Background(), tool.Call{
		Input:     map[string]any{"staged": true},
		Workspace: "/tmp",
	})
	if err != nil {
		t.Fatalf("failed to execute git_diff: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %v", res.Message)
	}
	if res.Output != "mock diff output" {
		t.Errorf("expected diff output, got %q", res.Output)
	}
	if len(res.FilesChanged) != 2 || res.FilesChanged[0] != "main.go" || res.FilesChanged[1] != "internal/tool/tool.go" {
		t.Errorf("FilesChanged mismatch: %v", res.FilesChanged)
	}
}
