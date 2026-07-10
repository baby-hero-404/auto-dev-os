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

// ListFilesTool implements tool.Tool to list files in a tree structure.
type ListFilesTool struct{}

// Name returns the unique tool name.
func (t *ListFilesTool) Name() string { return "list_files" }

// Category returns the tool's category.
func (t *ListFilesTool) Category() tool.Category { return tool.CategoryFilesystem }

// Capabilities returns the capability permissions required.
func (t *ListFilesTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapRead} }

// Description returns a description for the LLM.
func (t *ListFilesTool) Description() string {
	return "List workspace files and directories in a structured tree format up to a maximum depth."
}

// Schema returns the JSON schema for tool inputs.
func (t *ListFilesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":      {"type": "string", "default": ".", "description": "Workspace-relative starting path"},
			"max_depth": {"type": "integer", "default": 3, "description": "Maximum directory depth to traverse"},
			"max_files": {"type": "integer", "default": 200, "description": "Maximum number of files/dirs to display"}
		}
	}`)
}

type ListFilesArgs struct {
	Path     string `json:"path"`
	MaxDepth int    `json:"max_depth"`
	MaxFiles int    `json:"max_files"`
}

// Execute runs the file listing operation.
func (t *ListFilesTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var args ListFilesArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxDepth <= 0 {
		args.MaxDepth = 3
	}
	if args.MaxFiles <= 0 {
		args.MaxFiles = 200
	}

	targetPath, err := tool.SafeWorkspacePath(call.Workspace, args.Path)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: err.Error()},
			},
		}, nil
	}

	fileCount := 0
	limitReached := false
	var lines []string
	var flatFiles []string

	var traverse func(path string, depth int, prefix string)
	traverse = func(path string, depth int, prefix string) {
		if depth > args.MaxDepth {
			return
		}
		if fileCount >= args.MaxFiles {
			limitReached = true
			return
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}

		// Filter entries first
		var filtered []os.DirEntry
		for _, entry := range entries {
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
				continue
			}
			filtered = append(filtered, entry)
		}

		for i, entry := range filtered {
			if fileCount >= args.MaxFiles {
				limitReached = true
				break
			}
			fileCount++

			isLast := i == len(filtered)-1
			connector := "├── "
			nextPrefix := prefix + "│   "
			if isLast {
				connector = "└── "
				nextPrefix = prefix + "    "
			}

			name := entry.Name()
			display := name
			if entry.IsDir() {
				display = name + "/"
			} else {
				if rel, err := filepath.Rel(call.Workspace, filepath.Join(path, name)); err == nil {
					flatFiles = append(flatFiles, filepath.ToSlash(rel))
				}
			}

			lines = append(lines, prefix+connector+display)

			if entry.IsDir() {
				traverse(filepath.Join(path, name), depth+1, nextPrefix)
			}
		}
	}

	baseName := filepath.Base(targetPath)
	if baseName == "." || baseName == "/" || args.Path == "." {
		baseName = "."
	}
	lines = append(lines, baseName)

	traverse(targetPath, 1, "")

	if limitReached {
		lines = append(lines, fmt.Sprintf("... (maximum limit of %d files reached)", args.MaxFiles))
	}

	output := strings.Join(lines, "\n")

	return tool.Result{
		Success: true,
		Output:  output,
		Metadata: map[string]any{
			"file_count":    fileCount,
			"limit_reached": limitReached,
			"files":         flatFiles,
		},
	}, nil
}
