package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// GitRestoreTool implements tool.Tool to restore workspace using git reset.
type GitRestoreTool struct {
	Runtime sandbox.Runtime
}

// NewGitRestoreTool creates a new GitRestoreTool.
func NewGitRestoreTool(runtime sandbox.Runtime) *GitRestoreTool {
	return &GitRestoreTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *GitRestoreTool) Name() string { return "git_restore" }

// Category returns the tool's category.
func (t *GitRestoreTool) Category() tool.Category { return tool.CategoryGit }

// Capabilities returns the capability permissions required.
func (t *GitRestoreTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapGit} }

// Description returns a description for the LLM.
func (t *GitRestoreTool) Description() string {
	return "Restore the workspace to a previously saved checkpoint (commit hash) using git reset and clean."
}

// Schema returns the JSON schema for tool inputs.
func (t *GitRestoreTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["commit_hash"],
		"properties": {
			"commit_hash": {"type": "string", "description": "The commit hash of the checkpoint to restore to"}
		}
	}`)
}

type GitRestoreArgs struct {
	CommitHash string `json:"commit_hash"`
}

// Execute restores the workspace to the specified commit hash.
func (t *GitRestoreTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	var args GitRestoreArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.CommitHash == "" {
		return tool.Result{Success: false, Message: "missing required 'commit_hash' parameter"}, nil
	}

	commitHash := strings.TrimSpace(args.CommitHash)
	if strings.ContainsAny(commitHash, ";&|`\n$") {
		return tool.Result{Success: false, Message: "invalid commit hash"}, nil
	}

	// 1. Git reset
	resetRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"git", "reset", "--hard", commitHash},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to git reset: %w", err)
	}

	if resetRes.ExitCode != 0 {
		return tool.Result{
			Success: false,
			Output:  resetRes.Stdout + "\n" + resetRes.Stderr,
			Message: fmt.Sprintf("git reset to %s failed", commitHash),
		}, nil
	}

	// 2. Git clean
	cleanRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"git", "clean", "-fd"},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to git clean: %w", err)
	}

	output := fmt.Sprintf("Reset output:\n%s\n\nClean output:\n%s", resetRes.Stdout, cleanRes.Stdout)
	return tool.Result{
		Success: true,
		Output:  output,
		Metadata: map[string]any{
			"restored_hash": commitHash,
		},
	}, nil
}
