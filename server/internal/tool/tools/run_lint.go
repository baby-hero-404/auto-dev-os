package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// RunLintTool implements tool.Tool to execute golangci-lint in the sandbox.
type RunLintTool struct {
	Runtime sandbox.Runtime
}

// NewRunLintTool creates a new RunLintTool.
func NewRunLintTool(runtime sandbox.Runtime) *RunLintTool {
	return &RunLintTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *RunLintTool) Name() string { return "run_lint" }

// Category returns the tool's category.
func (t *RunLintTool) Category() tool.Category { return tool.CategoryBuild }

// Capabilities returns the capability permissions required.
func (t *RunLintTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapBuild} }

// Description returns a description for the LLM.
func (t *RunLintTool) Description() string {
	return "Run the golangci-lint linter in the workspace and parse the diagnostics."
}

// Schema returns the JSON schema for tool inputs.
func (t *RunLintTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Optional relative subdirectory to run the linter in"}
		}
	}`)
}

type RunLintArgs struct {
	Path string `json:"path"`
}

type golangciLintOutput struct {
	Issues []struct {
		FromLinter string `json:"FromLinter"`
		Text       string `json:"Text"`
		Pos        struct {
			Filename string `json:"Filename"`
			Line     int    `json:"Line"`
			Column   int    `json:"Column"`
		} `json:"Pos"`
	} `json:"Issues"`
}

// Execute runs the lint command in the sandbox and parses its output.
func (t *RunLintTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	var args RunLintArgs
	argsBytes, err := json.Marshal(call.Input)
	if err == nil {
		_ = json.Unmarshal(argsBytes, &args)
	}

	cmd := "golangci-lint run --out-format=json"
	var runCmd []string
	if args.Path != "" {
		safePath, err := tool.SafeWorkspacePath(call.Workspace, args.Path)
		if err != nil {
			return tool.Result{
				Success:     false,
				Diagnostics: []tool.Diagnostic{{Severity: "error", Message: err.Error()}},
			}, nil
		}
		runCmd = []string{"sh", "-c", fmt.Sprintf("cd %s && %s", safePath, cmd)}
	} else {
		runCmd = []string{"sh", "-c", cmd}
	}

	res, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   runCmd,
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to run linter: %w", err)
	}

	var diagnostics []tool.Diagnostic
	var lOutput golangciLintOutput

	// Try parsing JSON output
	if err := json.Unmarshal([]byte(res.Stdout), &lOutput); err == nil {
		for _, issue := range lOutput.Issues {
			diagnostics = append(diagnostics, tool.Diagnostic{
				Severity: "error",
				File:     issue.Pos.Filename,
				Line:     issue.Pos.Line,
				Message:  fmt.Sprintf("[%s] %s", issue.FromLinter, issue.Text),
			})
		}
	} else {
		// Fallback for non-JSON output, parse line by line
		lines := strings.Split(res.Stdout+"\n"+res.Stderr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			diagnostics = append(diagnostics, tool.Diagnostic{
				Severity: "error",
				Message:  line,
			})
		}
	}

	success := res.ExitCode == 0

	return tool.Result{
		Success:     success,
		Output:      res.Stdout + "\n" + res.Stderr,
		Diagnostics: diagnostics,
	}, nil
}
