package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// GitDiffTool implements tool.Tool to run git diff in the sandbox.
type GitDiffTool struct {
	Runtime sandbox.Runtime
}

// NewGitDiffTool creates a new GitDiffTool.
func NewGitDiffTool(runtime sandbox.Runtime) *GitDiffTool {
	return &GitDiffTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *GitDiffTool) Name() string { return "git_diff" }

// Category returns the tool's category.
func (t *GitDiffTool) Category() tool.Category { return tool.CategoryGit }

// Capabilities returns the capability permissions required.
func (t *GitDiffTool) Capabilities() []tool.Capability {
	return []tool.Capability{tool.CapGit, tool.CapGitDiff}
}

// Description returns a description for the LLM.
func (t *GitDiffTool) Description() string {
	return "Show unstaged or staged git diff changes for a file or the entire workspace."
}

// Schema returns the JSON schema for tool inputs.
func (t *GitDiffTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"staged": {"type": "boolean", "default": false, "description": "Show only staged changes"},
			"path":   {"type": "string", "description": "Optional workspace-relative file path to filter the diff"}
		}
	}`)
}

// Execute runs the git diff command in the sandbox.
func (t *GitDiffTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	staged, _ := call.Input["staged"].(bool)
	pathFilter, _ := call.Input["path"].(string)

	// Run git diff output command
	diffCmd := []string{"git", "diff"}
	if staged {
		diffCmd = append(diffCmd, "--staged")
	}
	if pathFilter != "" {
		diffCmd = append(diffCmd, pathFilter)
	}

	diffRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   diffCmd,
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to run git diff: %w", err)
	}

	if diffRes.ExitCode != 0 {
		return tool.Result{
			Success: false,
			Output:  diffRes.Stderr,
			Message: fmt.Sprintf("git diff failed with exit code %d", diffRes.ExitCode),
		}, nil
	}

	// Run name-only command to get list of files changed
	nameCmd := []string{"git", "diff"}
	if staged {
		nameCmd = append(nameCmd, "--staged")
	}
	nameCmd = append(nameCmd, "--name-only")
	if pathFilter != "" {
		nameCmd = append(nameCmd, pathFilter)
	}

	nameRes, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   nameCmd,
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to run git diff --name-only: %w", err)
	}

	var filesChanged []string
	if nameRes.ExitCode == 0 {
		for _, line := range strings.Split(nameRes.Stdout, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				filesChanged = append(filesChanged, line)
			}
		}
	}

	output := diffRes.Stdout
	if output == "" {
		output = "No changes."
	}

	return tool.Result{
		Success:      true,
		Output:       output,
		FilesChanged: filesChanged,
	}, nil
}
