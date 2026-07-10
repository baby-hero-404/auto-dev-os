package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// RunTestsTool implements tool.Tool to run tests inside the sandbox.
type RunTestsTool struct {
	Runtime sandbox.Runtime
}

// NewRunTestsTool creates a new RunTestsTool.
func NewRunTestsTool(runtime sandbox.Runtime) *RunTestsTool {
	return &RunTestsTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *RunTestsTool) Name() string { return "run_tests" }

// Category returns the tool's category.
func (t *RunTestsTool) Category() tool.Category { return tool.CategoryBuild }

// Capabilities returns the capability permissions required.
func (t *RunTestsTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapBuild} }

// Description returns a description for the LLM.
func (t *RunTestsTool) Description() string {
	return "Run tests in the workspace and parse the output for pass/fail diagnostics."
}

// Schema returns the JSON schema for tool inputs.
func (t *RunTestsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "default": "go test ./...", "description": "Test command to execute"},
			"path":    {"type": "string", "description": "Optional workspace-relative directory to run tests in"}
		}
	}`)
}

// Execute runs the test command in the sandbox and parses its output.
func (t *RunTestsTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	command, _ := call.Input["command"].(string)
	if command == "" {
		command = "go test ./..."
	}
	path, _ := call.Input["path"].(string)

	var runCmd []string
	if path != "" {
		// Ensure path is safe/clean
		safePath, err := tool.SafeWorkspacePath(call.Workspace, path)
		if err != nil {
			return tool.Result{
				Success: false,
				Diagnostics: []tool.Diagnostic{
					{Severity: "error", Message: err.Error()},
				},
			}, nil
		}
		// Convert absolute path back to workspace-relative or use it directly in cd
		runCmd = []string{"sh", "-c", fmt.Sprintf("cd %s && %s", safePath, command)}
	} else {
		runCmd = []string{"sh", "-c", command}
	}

	res, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   runCmd,
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to run tests: %w", err)
	}

	var diagnostics []tool.Diagnostic
	lines := strings.Split(res.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--- FAIL:") {
			testName := strings.TrimSpace(strings.TrimPrefix(line, "--- FAIL:"))
			if idx := strings.Index(testName, " "); idx != -1 {
				testName = testName[:idx]
			}
			diagnostics = append(diagnostics, tool.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("Test %s failed", testName),
			})
		} else if strings.HasPrefix(line, "--- PASS:") {
			testName := strings.TrimSpace(strings.TrimPrefix(line, "--- PASS:"))
			if idx := strings.Index(testName, " "); idx != -1 {
				testName = testName[:idx]
			}
			diagnostics = append(diagnostics, tool.Diagnostic{
				Severity: "info",
				Message:  fmt.Sprintf("Test %s passed", testName),
			})
		}
	}

	// Also scan stderr for compiler / setup failures
	if res.ExitCode != 0 && len(diagnostics) == 0 {
		diagnostics = append(diagnostics, tool.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("Test command failed with exit status %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr)),
		})
	}

	success := res.ExitCode == 0

	return tool.Result{
		Success:     success,
		Output:      res.Stdout + "\n" + res.Stderr,
		Diagnostics: diagnostics,
	}, nil
}
