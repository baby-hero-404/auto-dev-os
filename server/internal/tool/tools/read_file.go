package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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
	return "Read contents of one or more workspace files (use 'paths' to batch multiple files into a single call). Supports reading the full file or targeting specific line ranges/contexts. Output lines are prefixed with 1-indexed line numbers."
}

// Schema returns the JSON schema for tool inputs.
func (t *ReadFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":        {"type": "string", "description": "Workspace-relative file path (single-file mode)"},
			"paths":       {"type": "array", "items": {"type": "string"}, "description": "Multiple workspace-relative file paths to read in one call (batch mode). Line-range/around_line args apply identically to every path."},
			"start_line":  {"type": "integer", "description": "1-indexed start line (inclusive)"},
			"end_line":    {"type": "integer", "description": "1-indexed end line (inclusive)"},
			"around_line": {"type": "integer", "description": "1-indexed center line to read around"},
			"radius":      {"type": "integer", "description": "Number of lines to read above and below around_line"},
			"max_lines":   {"type": "integer", "default": 500, "description": "Maximum number of lines to return per file when reading full file"}
		}
	}`)
}

type ReadFileArgs struct {
	Path       string   `json:"path"`
	Paths      []string `json:"paths"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	AroundLine int      `json:"around_line"`
	Radius     int      `json:"radius"`
	MaxLines   int      `json:"max_lines"`
}

type readFileOutcome struct {
	path          string
	output        string
	totalLines    int
	returnedLines int
	startLine     int
	endLine       int
	err           string
}

// Execute runs the file reading operation for one or more files.
func (t *ReadFileTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var args ReadFileArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	paths := args.Paths
	if args.Path != "" {
		paths = append([]string{args.Path}, paths...)
	}
	if len(paths) == 0 {
		return tool.Result{Success: false, Message: "missing required 'path' or 'paths' parameter"}, nil
	}

	outcomes := make([]readFileOutcome, 0, len(paths))
	var diagnostics []tool.Diagnostic
	anySuccess := false

	for _, p := range paths {
		outcome := t.readOne(call, args, p)
		if outcome.err != "" {
			diagnostics = append(diagnostics, tool.Diagnostic{Severity: "error", File: p, Message: outcome.err})
		} else {
			anySuccess = true
		}
		outcomes = append(outcomes, outcome)
	}

	metadata := map[string]any{
		"files": buildFileMetadata(outcomes),
	}

	// Single-file mode keeps the flat metadata shape callers already depend on.
	if len(outcomes) == 1 {
		o := outcomes[0]
		metadata["total_lines"] = o.totalLines
		metadata["returned_lines"] = o.returnedLines
		metadata["start_line"] = o.startLine
		metadata["end_line"] = o.endLine
		return tool.Result{
			Success:     o.err == "",
			Output:      o.output,
			Diagnostics: diagnostics,
			Metadata:    metadata,
		}, nil
	}

	return tool.Result{
		Success:     anySuccess,
		Output:      joinBatchOutput(outcomes),
		Diagnostics: diagnostics,
		Metadata:    metadata,
	}, nil
}

func buildFileMetadata(outcomes []readFileOutcome) []map[string]any {
	files := make([]map[string]any, 0, len(outcomes))
	for _, o := range outcomes {
		files = append(files, map[string]any{
			"path":           o.path,
			"total_lines":    o.totalLines,
			"returned_lines": o.returnedLines,
			"start_line":     o.startLine,
			"end_line":       o.endLine,
			"error":          o.err,
		})
	}
	return files
}

func joinBatchOutput(outcomes []readFileOutcome) string {
	var b strings.Builder
	for i, o := range outcomes {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "### %s\n", o.path)
		if o.err != "" {
			fmt.Fprintf(&b, "error: %s", o.err)
			continue
		}
		b.WriteString(o.output)
	}
	return b.String()
}

func (t *ReadFileTool) readOne(call tool.Call, args ReadFileArgs, path string) readFileOutcome {
	outcome := readFileOutcome{path: path}

	fullPath, err := tool.SafeWorkspacePath(call.Workspace, path)
	if err != nil {
		outcome.err = err.Error()
		return outcome
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		outcome.err = fmt.Sprintf("cannot read file: %v", err)
		return outcome
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
		radius := max(args.Radius, 0)
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

	outcome.output = formatNumberedLines(returnedLines, start)
	outcome.totalLines = totalLines
	outcome.returnedLines = len(returnedLines)
	outcome.startLine = start
	outcome.endLine = end
	return outcome
}

// formatNumberedLines prefixes each line with its 1-indexed line number,
// starting at firstLineNo, in a "cat -n"-style right-aligned gutter.
func formatNumberedLines(lines []string, firstLineNo int) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strconv.Itoa(firstLineNo + i))
		b.WriteByte('\t')
		b.WriteString(line)
	}
	return b.String()
}
