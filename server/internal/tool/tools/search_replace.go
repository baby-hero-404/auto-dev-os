package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// SearchReplaceTool implements tool.Tool to search and replace text in a file.
type SearchReplaceTool struct {
	Verify *tool.VerifyPipeline
}

// NewSearchReplaceTool creates a new SearchReplaceTool.
func NewSearchReplaceTool(verify *tool.VerifyPipeline) *SearchReplaceTool {
	return &SearchReplaceTool{Verify: verify}
}

// Name returns the unique tool name.
func (t *SearchReplaceTool) Name() string { return "search_replace" }

// Category returns the tool's category.
func (t *SearchReplaceTool) Category() tool.Category { return tool.CategoryEditing }

// Capabilities returns the capability permissions required.
func (t *SearchReplaceTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapEdit} }

// Description returns a description for the LLM.
func (t *SearchReplaceTool) Description() string {
	return "Apply a targeted search-and-replace edit to a file. Finds the exact 'search' block and replaces it with 'replace'. Supports dry_run mode to preview changes."
}

// Schema returns the JSON schema for tool inputs.
func (t *SearchReplaceTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["path", "search", "replace"],
		"properties": {
			"path":    {"type": "string", "description": "Workspace-relative file path"},
			"search":  {"type": "string", "description": "Exact text to find (must match once)"},
			"replace": {"type": "string", "description": "Replacement text"},
			"dry_run": {"type": "boolean", "default": false, "description": "Preview without applying"},
			"verify":  {"type": "boolean", "default": true, "description": "Run post-edit verification"}
		}
	}`)
}

// Execute runs the search and replace operation.
func (t *SearchReplaceTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	path, _ := call.Input["path"].(string)
	search, _ := call.Input["search"].(string)
	replace, _ := call.Input["replace"].(string)
	dryRun, _ := call.Input["dry_run"].(bool)

	verify := true
	if v, ok := call.Input["verify"].(bool); ok {
		verify = v
	}

	if path == "" {
		return tool.Result{Success: false, Message: "missing required 'path' parameter"}, nil
	}
	if search == "" {
		return tool.Result{Success: false, Message: "missing required 'search' parameter"}, nil
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

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: path, Message: fmt.Sprintf("cannot read file: %v", err)},
			},
		}, nil
	}

	content := string(data)
	hashBefore := tool.Sha256Hash(data)

	count := strings.Count(content, search)
	if count == 0 {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: path, Message: "search block not found in file"},
			},
		}, nil
	}
	if count > 1 {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: path, Message: fmt.Sprintf("ambiguous: search block found %d times", count)},
			},
		}, nil
	}

	updated := strings.Replace(content, search, replace, 1)

	if dryRun {
		return tool.Result{
			Success: true,
			Message: "dry run - no changes applied",
			Metadata: map[string]any{
				"diff_preview": tool.GenerateDiffPreview(search, replace, path),
			},
		}, nil
	}

	if err := os.WriteFile(fullPath, []byte(updated), 0o644); err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", File: path, Message: fmt.Sprintf("write failed: %v", err)},
			},
		}, nil
	}

	hashAfter := tool.Sha256Hash([]byte(updated))

	if verify && t.Verify != nil {
		diags := t.Verify.Run(ctx, call.Workspace, []string{path})
		hasError := false
		for _, d := range diags {
			if d.Severity == "error" {
				hasError = true
				break
			}
		}

		if hasError {
			// Rollback to original content
			_ = os.WriteFile(fullPath, data, 0o644)
			return tool.Result{
				Success:     false,
				Message:     "edit rolled back due to verification failure",
				Diagnostics: diags,
			}, nil
		}
	}

	return tool.Result{
		Success:      true,
		Message:      fmt.Sprintf("replaced in %s", path),
		FilesChanged: []string{path},
		Metadata: map[string]any{
			"replaced_count": 1,
			"hash_before":    hashBefore,
			"hash_after":     hashAfter,
		},
	}, nil
}
