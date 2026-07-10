package tools

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestGitStatusTool(t *testing.T) {
	porcelainOutput := "M  staged_modified.go\nA  staged_added.go\n M unstaged_modified.go\n?? untracked.go\n"
	mr := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			return &sandbox.CommandResult{
				ExitCode: 0,
				Stdout:   porcelainOutput,
			}, nil
		},
	}

	gst := NewGitStatusTool(mr)

	res, err := gst.Execute(context.Background(), tool.Call{Workspace: "/tmp"})
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %v", res.Message)
	}

	staged, _ := res.Metadata["staged"].([]string)
	unstaged, _ := res.Metadata["unstaged"].([]string)
	untracked, _ := res.Metadata["untracked"].([]string)

	if len(staged) != 2 || staged[0] != "staged_modified.go" || staged[1] != "staged_added.go" {
		t.Errorf("expected staged mismatch: %v", staged)
	}
	if len(unstaged) != 1 || unstaged[0] != "unstaged_modified.go" {
		t.Errorf("expected unstaged mismatch: %v", unstaged)
	}
	if len(untracked) != 1 || untracked[0] != "untracked.go" {
		t.Errorf("expected untracked mismatch: %v", untracked)
	}
}
