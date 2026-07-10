package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// ReadSpecTool implements tool.Tool to read files under docs/openspecs/<task>/.
type ReadSpecTool struct{}

// Name returns the unique tool name.
func (t *ReadSpecTool) Name() string { return "read_spec" }

// Category returns the tool's category.
func (t *ReadSpecTool) Category() tool.Category { return tool.CategoryContext }

// Capabilities returns the capability permissions required.
func (t *ReadSpecTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapContext} }

// Description returns a description for the LLM.
func (t *ReadSpecTool) Description() string {
	return "Read and return the specification files located in docs/openspecs/<task>/."
}

// Schema returns the JSON schema for tool inputs.
func (t *ReadSpecTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["task"],
		"properties": {
			"task": {"type": "string", "description": "The folder slug of the task specification under docs/openspecs/"}
		}
	}`)
}

// Execute reads the spec files.
func (t *ReadSpecTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	task, _ := call.Input["task"].(string)
	if task == "" {
		task = call.TaskID
	}
	if task == "" {
		return tool.Result{Success: false, Message: "missing required 'task' parameter and call.TaskID is empty"}, nil
	}

	// Clean path to prevent path injection
	task = filepath.Clean(task)
	if strings.Contains(task, "..") || strings.HasPrefix(task, "/") {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: "invalid task name format"},
			},
		}, nil
	}

	specRelPath := filepath.Join("docs", "openspecs", task)
	specPath, err := tool.SafeWorkspacePath(call.Workspace, specRelPath)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: err.Error()},
			},
		}, nil
	}

	entries, err := os.ReadDir(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tool.Result{
				Success: false,
				Diagnostics: []tool.Diagnostic{
					{Severity: "error", Message: fmt.Sprintf("spec directory %s does not exist", specRelPath)},
				},
			}, nil
		}
		return tool.Result{Success: false, Message: err.Error()}, nil
	}

	var outputBuilder strings.Builder
	readCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		filePath := filepath.Join(specPath, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		readCount++
		outputBuilder.WriteString(fmt.Sprintf("=== File: %s ===\n", name))
		outputBuilder.Write(data)
		outputBuilder.WriteString("\n\n")
	}

	if readCount == 0 {
		return tool.Result{
			Success: true,
			Output:  "No markdown specification files found in the directory.",
		}, nil
	}

	return tool.Result{
		Success: true,
		Output:  outputBuilder.String(),
		Metadata: map[string]any{
			"spec_files_read": readCount,
		},
	}, nil
}
