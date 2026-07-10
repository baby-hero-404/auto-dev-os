package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// FileExistsTool implements tool.Tool to check if a file exists.
type FileExistsTool struct{}

// Name returns the unique tool name.
func (t *FileExistsTool) Name() string { return "file_exists" }

// Category returns the tool's category.
func (t *FileExistsTool) Category() tool.Category { return tool.CategoryFilesystem }

// Capabilities returns the capability permissions required.
func (t *FileExistsTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapRead} }

// Description returns a description for the LLM.
func (t *FileExistsTool) Description() string {
	return "Check if a file or directory exists in the workspace and retrieve its size and type."
}

// Schema returns the JSON schema for tool inputs.
func (t *FileExistsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string", "description": "Workspace-relative file or directory path"}
		}
	}`)
}

// Execute runs the file checking operation.
func (t *FileExistsTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	path, _ := call.Input["path"].(string)
	if path == "" {
		return tool.Result{Success: false, Message: "missing required 'path' parameter"}, nil
	}

	fullPath, err := tool.SafeWorkspacePath(call.Workspace, path)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: err.Error()},
			},
		}, nil
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tool.Result{
				Success: true,
				Output:  fmt.Sprintf("Path %s does not exist.", path),
				Metadata: map[string]any{
					"exists": false,
					"size":   0,
					"is_dir": false,
				},
			}, nil
		}
		return tool.Result{Success: false, Message: err.Error()}, nil
	}

	return tool.Result{
		Success: true,
		Output:  fmt.Sprintf("Path %s exists (Directory: %t, Size: %d bytes).", path, info.IsDir(), info.Size()),
		Metadata: map[string]any{
			"exists": true,
			"size":   info.Size(),
			"is_dir": info.IsDir(),
		},
	}, nil
}
