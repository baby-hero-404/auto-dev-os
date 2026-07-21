package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// ReadAffectedFilesTool implements tool.Tool to read all estimated affected files.
type ReadAffectedFilesTool struct {
	Provider tool.AffectedFilesProvider
}

// NewReadAffectedFilesTool creates a new ReadAffectedFilesTool.
func NewReadAffectedFilesTool(provider tool.AffectedFilesProvider) *ReadAffectedFilesTool {
	return &ReadAffectedFilesTool{Provider: provider}
}

// Name returns the unique tool name.
func (t *ReadAffectedFilesTool) Name() string { return "read_affected_files" }

// Category returns the tool's category.
func (t *ReadAffectedFilesTool) Category() tool.Category { return tool.CategoryContext }

// Capabilities returns the capability permissions required.
func (t *ReadAffectedFilesTool) Capabilities() []tool.Capability {
	return []tool.Capability{tool.CapContext}
}

// Description returns a description for the LLM.
func (t *ReadAffectedFilesTool) Description() string {
	return "Read and return the contents of all files estimated to be affected by the current task."
}

// Schema returns the JSON schema for tool inputs.
func (t *ReadAffectedFilesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute runs the read affected files operation.
func (t *ReadAffectedFilesTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Provider == nil {
		return tool.Result{Success: false, Message: "affected files provider is not configured"}, nil
	}

	taskID := call.TaskID
	if taskID == "" {
		return tool.Result{Success: false, Message: "missing required call.TaskID"}, nil
	}

	files, err := t.Provider.GetAffectedFiles(ctx, taskID)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: fmt.Sprintf("failed to get affected files: %v", err)},
			},
		}, nil
	}

	var outputBuilder strings.Builder
	var readFiles []string

	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}

		fullPath, err := tool.SafeWorkspacePath(call.Workspace, f)
		if err != nil {
			// Skip files outside workspace or log as diagnostic
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			// Skip unreadable files
			continue
		}

		readFiles = append(readFiles, f)
		outputBuilder.WriteString(fmt.Sprintf("=== File: %s ===\n", f))
		outputBuilder.Write(data)
		outputBuilder.WriteString("\n\n")
	}

	if len(readFiles) == 0 {
		return tool.Result{
			Success: true,
			Output:  "No affected files could be read.",
			Metadata: map[string]any{
				"affected_files": files,
				"files_read":     []string{},
			},
		}, nil
	}

	return tool.Result{
		Success: true,
		Output:  outputBuilder.String(),
		Metadata: map[string]any{
			"affected_files": files,
			"files_read":     readFiles,
		},
	}, nil
}
