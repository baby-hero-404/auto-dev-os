package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// CreateFileTool implements tool.Tool to safely write new files without overwriting existing non-empty files.
type CreateFileTool struct {
	Verify *tool.VerifyPipeline
}

// NewCreateFileTool creates a new CreateFileTool.
func NewCreateFileTool(verify *tool.VerifyPipeline) *CreateFileTool {
	return &CreateFileTool{Verify: verify}
}

// Name returns the unique tool name.
func (t *CreateFileTool) Name() string { return "create_file" }

// Category returns the tool's category.
func (t *CreateFileTool) Category() tool.Category { return tool.CategoryFilesystem }

// Capabilities returns the capability permissions required.
func (t *CreateFileTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapCreate} }

// Description returns a description for the LLM.
func (t *CreateFileTool) Description() string {
	return "Create a new file in the workspace or write to an empty file. Refuses to overwrite existing non-empty files."
}

// Schema returns the JSON schema for tool inputs.
func (t *CreateFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["path", "content"],
		"properties": {
			"path":    {"type": "string", "description": "Workspace-relative file path"},
			"content": {"type": "string", "description": "Content to write into the file"},
			"verify":  {"type": "boolean", "default": true, "description": "Run post-creation verification"}
		}
	}`)
}

type CreateFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Verify  *bool  `json:"verify"`
}

// Execute safely creates the file and runs optional verification checks.
func (t *CreateFileTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var args CreateFileArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Path == "" {
		return tool.Result{Success: false, Message: "missing required 'path' parameter"}, nil
	}

	fullPath, err := tool.SafeWorkspacePath(call.Workspace, args.Path)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: err.Error()},
			},
		}, nil
	}

	existed := true
	stat, statErr := os.Stat(fullPath)
	if os.IsNotExist(statErr) {
		existed = false
	} else if stat != nil && stat.Size() > 0 {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: args.Path, Message: "file already exists and is not empty"},
			},
		}, nil
	}

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: args.Path, Message: fmt.Sprintf("failed to create directory structure: %v", err)},
			},
		}, nil
	}

	// Write content
	if err := os.WriteFile(fullPath, []byte(args.Content), 0o644); err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: args.Path, Message: fmt.Sprintf("write failed: %v", err)},
			},
		}, nil
	}

	verify := true
	if args.Verify != nil {
		verify = *args.Verify
	}

	if verify && t.Verify != nil {
		diags := t.Verify.Run(ctx, call.Workspace, []string{args.Path})
		hasError := false
		for _, d := range diags {
			if d.Severity == "error" {
				hasError = true
				break
			}
		}

		if hasError {
			// Rollback
			if existed {
				_ = os.WriteFile(fullPath, []byte{}, 0o644)
			} else {
				_ = os.Remove(fullPath)
			}
			return tool.Result{
				Success:     false,
				Message:     "file creation rolled back due to verification failure",
				Diagnostics: diags,
			}, nil
		}
	}

	return tool.Result{
		Success:      true,
		Message:      fmt.Sprintf("created file %s", args.Path),
		FilesChanged: []string{args.Path},
		Metadata: map[string]any{
			"existed_before": existed,
		},
	}, nil
}
