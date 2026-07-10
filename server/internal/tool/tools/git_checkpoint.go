package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// GitCheckpointTool implements tool.Tool to create a git commit checkpoint.
type GitCheckpointTool struct {
	Runtime sandbox.Runtime
}

// NewGitCheckpointTool creates a new GitCheckpointTool.
func NewGitCheckpointTool(runtime sandbox.Runtime) *GitCheckpointTool {
	return &GitCheckpointTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *GitCheckpointTool) Name() string { return "git_checkpoint" }

// Category returns the tool's category.
func (t *GitCheckpointTool) Category() tool.Category { return tool.CategoryGit }

// Capabilities returns the capability permissions required.
func (t *GitCheckpointTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapGit} }

// Description returns a description for the LLM.
func (t *GitCheckpointTool) Description() string {
	return "Create a checkpoint commit of the current workspace state. Returns the commit hash."
}

// Schema returns the JSON schema for tool inputs.
func (t *GitCheckpointTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["message"],
		"properties": {
			"message": {"type": "string", "description": "Commit message describing the checkpoint"}
		}
	}`)
}

type GitCheckpointArgs struct {
	Message string `json:"message"`
}

// Execute commits the current worktree changes and returns the new commit hash.
func (t *GitCheckpointTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	var args GitCheckpointArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Message == "" {
		return tool.Result{Success: false, Message: "missing required 'message' parameter"}, nil
	}

	// 1. Check if there are changes
	statusRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"git", "status", "--porcelain"},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to check git status: %w", err)
	}

	if strings.TrimSpace(statusRes.Stdout) == "" {
		revRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
			TaskID:    call.TaskID,
			AgentID:   call.AgentID,
			Workspace: call.Workspace,
			Command:   []string{"git", "rev-parse", "HEAD"},
		})
		if err != nil {
			return tool.Result{}, fmt.Errorf("failed to get HEAD: %w", err)
		}
		hash := strings.TrimSpace(revRes.Stdout)
		return tool.Result{
			Success: true,
			Output:  fmt.Sprintf("no changes to checkpoint; current HEAD is %s", hash),
			Metadata: map[string]any{
				"commit_hash": hash,
				"clean":       true,
			},
		}, nil
	}

	// 2. Stage and commit
	commitCmd := fmt.Sprintf("git add -A && git commit -m %q", args.Message)
	res, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"sh", "-c", commitCmd},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to commit: %w", err)
	}

	if res.ExitCode != 0 {
		return tool.Result{
			Success: false,
			Output:  res.Stdout + "\n" + res.Stderr,
			Message: "failed to create checkpoint commit",
		}, nil
	}

	// 3. Get the new commit hash
	revRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"git", "rev-parse", "HEAD"},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to get new HEAD: %w", err)
	}

	hash := strings.TrimSpace(revRes.Stdout)
	return tool.Result{
		Success: true,
		Output:  fmt.Sprintf("checkpoint created: %s", hash),
		Metadata: map[string]any{
			"commit_hash": hash,
			"clean":       false,
		},
	}, nil
}
