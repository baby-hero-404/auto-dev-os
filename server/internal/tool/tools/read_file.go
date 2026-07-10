package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// ReadFileTool implements tool.Tool to read files.
type ReadFileTool struct{}

// Name returns the unique tool name.
func (t *ReadFileTool) Name() string { return "read_file" }

// Category returns the tool's category.
func (t *ReadFileTool) Category() tool.Category { return tool.CategoryFilesystem }

// Capabilities returns the capability permissions required.
func (t *ReadFileTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapRead} }

// Description returns a description for the LLM.
func (t *ReadFileTool) Description() string {
	return "Read contents of a workspace file. Supports reading the full file or targeting specific line ranges/contexts."
}

// Schema returns the JSON schema for tool inputs.
func (t *ReadFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path":        {"type": "string", "description": "Workspace-relative file path"},
			"start_line":  {"type": "integer", "description": "1-indexed start line (inclusive)"},
			"end_line":    {"type": "integer", "description": "1-indexed end line (inclusive)"},
			"around_line": {"type": "integer", "description": "1-indexed center line to read around"},
			"radius":      {"type": "integer", "description": "Number of lines to read above and below around_line"},
			"max_lines":   {"type": "integer", "default": 500, "description": "Maximum number of lines to return when reading full file"}
		}
	}`)
}

type ReadFileArgs struct {
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	AroundLine int    `json:"around_line"`
	Radius     int    `json:"radius"`
	MaxLines   int    `json:"max_lines"`
}

// Execute runs the file reading operation.
func (t *ReadFileTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var args ReadFileArgs
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

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: args.Path, Message: fmt.Sprintf("cannot read file: %v", err)},
			},
		}, nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	maxLines := args.MaxLines
	if maxLines <= 0 {
		maxLines = 500
	}

	start := 1
	end := totalLines

	if args.AroundLine > 0 {
		radius := args.Radius
		if radius < 0 {
			radius = 0
		}
		start = args.AroundLine - radius
		end = args.AroundLine + radius
	} else if args.StartLine > 0 {
		start = args.StartLine
		if args.EndLine > 0 {
			end = args.EndLine
		} else {
			end = start + maxLines - 1
		}
	} else {
		start = 1
		end = maxLines
	}

	if start < 1 {
		start = 1
	}
	if end > totalLines {
		end = totalLines
	}
	if start > totalLines {
		start = totalLines
	}
	if end < start {
		end = start
	}

	var returnedLines []string
	if totalLines > 0 {
		returnedLines = lines[start-1 : end]
	}

	output := strings.Join(returnedLines, "\n")

	return tool.Result{
		Success: true,
		Output:  output,
		Metadata: map[string]any{
			"total_lines":    totalLines,
			"returned_lines": len(returnedLines),
			"start_line":     start,
			"end_line":       end,
		},
	}, nil
}
