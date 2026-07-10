package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// GitStatusTool implements tool.Tool to check git status in the sandbox.
type GitStatusTool struct {
	Runtime sandbox.Runtime
}

// NewGitStatusTool creates a new GitStatusTool.
func NewGitStatusTool(runtime sandbox.Runtime) *GitStatusTool {
	return &GitStatusTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *GitStatusTool) Name() string { return "git_status" }

// Category returns the tool's category.
func (t *GitStatusTool) Category() tool.Category { return tool.CategoryGit }

// Capabilities returns the capability permissions required.
func (t *GitStatusTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapGit} }

// Description returns a description for the LLM.
func (t *GitStatusTool) Description() string {
	return "Show the working tree status of the git repository."
}

// Schema returns the JSON schema for tool inputs.
func (t *GitStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute runs the git status command and parses the output.
func (t *GitStatusTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	res, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"git", "status", "--porcelain"},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to run git status: %w", err)
	}

	if res.ExitCode != 0 {
		return tool.Result{
			Success: false,
			Output:  res.Stderr,
			Message: fmt.Sprintf("git status failed with exit code %d", res.ExitCode),
		}, nil
	}

	var staged []string
	var unstaged []string
	var untracked []string

	lines := strings.Split(res.Stdout, "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		status := line[0:2]
		filePart := line[3:]

		// Handle renames
		if strings.Contains(filePart, " -> ") {
			parts := strings.Split(filePart, " -> ")
			filePart = parts[len(parts)-1]
		}
		filePart = strings.Trim(filePart, "\"")

		if status == "??" {
			untracked = append(untracked, filePart)
		} else {
			if status[0] != ' ' {
				staged = append(staged, filePart)
			}
			if status[1] != ' ' {
				unstaged = append(unstaged, filePart)
			}
		}
	}

	output := res.Stdout
	if output == "" {
		output = "Working tree clean."
	}

	return tool.Result{
		Success: true,
		Output:  output,
		Metadata: map[string]any{
			"staged":    staged,
			"unstaged":  unstaged,
			"untracked": untracked,
		},
	}, nil
}
