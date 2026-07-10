package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

var goCompilerErrorRegex = regexp.MustCompile(`^([^:\s]+):(\d+):(?:\d+:)?\s*(.*)$`)

// RunBuildTool implements tool.Tool to run build command and parse errors.
type RunBuildTool struct {
	Runtime sandbox.Runtime
}

// NewRunBuildTool creates a new RunBuildTool.
func NewRunBuildTool(runtime sandbox.Runtime) *RunBuildTool {
	return &RunBuildTool{Runtime: runtime}
}

// Name returns the unique tool name.
func (t *RunBuildTool) Name() string { return "run_build" }

// Category returns the tool's category.
func (t *RunBuildTool) Category() tool.Category { return tool.CategoryBuild }

// Capabilities returns the capability permissions required.
func (t *RunBuildTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapBuild} }

// Description returns a description for the LLM.
func (t *RunBuildTool) Description() string {
	return "Execute a build command in the workspace and parse compilation errors into file and line diagnostics."
}

// Schema returns the JSON schema for tool inputs.
func (t *RunBuildTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "default": "go build ./...", "description": "Build command to execute"}
		}
	}`)
}

// Execute runs the build command and parses any errors.
func (t *RunBuildTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	command, _ := call.Input["command"].(string)
	if command == "" {
		command = "go build ./..."
	}

	res, err := t.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
		Workspace: call.Workspace,
		Command:   []string{"sh", "-c", command},
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to run build: %w", err)
	}

	var diagnostics []tool.Diagnostic

	// Parse stdout and stderr for compiler errors
	outputLines := strings.Split(res.Stdout+"\n"+res.Stderr, "\n")
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := goCompilerErrorRegex.FindStringSubmatch(line)
		if len(matches) == 4 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])
			message := matches[3]
			diagnostics = append(diagnostics, tool.Diagnostic{
				Severity: "error",
				File:     file,
				Line:     lineNum,
				Message:  message,
			})
		}
	}

	if res.ExitCode != 0 && len(diagnostics) == 0 {
		diagnostics = append(diagnostics, tool.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("Build command failed with exit status %d", res.ExitCode),
		})
	}

	success := res.ExitCode == 0

	return tool.Result{
		Success:     success,
		Output:      res.Stdout + "\n" + res.Stderr,
		Diagnostics: diagnostics,
	}, nil
}
