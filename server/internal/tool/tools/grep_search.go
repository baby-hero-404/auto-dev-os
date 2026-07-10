package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// GrepSearchTool implements tool.Tool to search for text or regex in workspace files.
type GrepSearchTool struct{}

// Name returns the unique tool name.
func (t *GrepSearchTool) Name() string { return "grep_search" }

// Category returns the tool's category.
func (t *GrepSearchTool) Category() tool.Category { return tool.CategorySearch }

// Capabilities returns the capability permissions required.
func (t *GrepSearchTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapSearch} }

// Description returns a description for the LLM.
func (t *GrepSearchTool) Description() string {
	return "Search workspace files for a literal query string or regular expression, returning matching lines with their line numbers."
}

// Schema returns the JSON schema for tool inputs.
func (t *GrepSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query":       {"type": "string", "description": "Literal text or regular expression to search for"},
			"regex":       {"type": "boolean", "default": false, "description": "Treat query as a regular expression"},
			"include":     {"type": "string", "description": "Glob pattern of files to include"},
			"exclude":     {"type": "string", "description": "Glob pattern of files to exclude"},
			"max_results": {"type": "integer", "default": 30, "description": "Maximum number of results to return"}
		}
	}`)
}

type GrepSearchArgs struct {
	Query      string `json:"query"`
	Regex      bool   `json:"regex"`
	Include    string `json:"include"`
	Exclude    string `json:"exclude"`
	MaxResults int    `json:"max_results"`
}

// Execute runs the grep search operation.
func (t *GrepSearchTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var args GrepSearchArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Query == "" {
		return tool.Result{Success: false, Message: "missing required 'query' parameter"}, nil
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 30
	}

	absWorkspace, err := filepath.Abs(call.Workspace)
	if err != nil {
		return tool.Result{
			Success: false,
			Diagnostics: []tool.Diagnostic{
				{Severity: "error", Message: fmt.Sprintf("invalid workspace: %v", err)},
			},
		}, nil
	}

	var re *regexp.Regexp
	if args.Regex {
		compiled, err := regexp.Compile(args.Query)
		if err != nil {
			return tool.Result{
				Success: false,
				Diagnostics: []tool.Diagnostic{
					{Severity: "error", Message: fmt.Sprintf("invalid regex pattern: %v", err)},
				},
			}, nil
		}
		re = compiled
	}

	var results []string
	count := 0

	err = filepath.Walk(absWorkspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(absWorkspace, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Filter by include/exclude
		if args.Exclude != "" && matchesGlob(args.Exclude, relPath) {
			return nil
		}
		if args.Include != "" && !matchesGlob(args.Include, relPath) {
			return nil
		}

		// Read and search file
		file, err := os.Open(path)
		if err != nil {
			return nil // ignore unreadable files
		}
		defer file.Close()

		// Skip binary files (approximate check: first 512 bytes)
		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		for i := 0; i < n; i++ {
			if buffer[i] == 0 {
				return nil // likely binary
			}
		}
		_, _ = file.Seek(0, 0)

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			if count >= maxResults {
				break
			}
			lineNum++
			line := scanner.Text()

			matched := false
			if args.Regex {
				matched = re.MatchString(line)
			} else {
				matched = strings.Contains(line, args.Query)
			}

			if matched {
				results = append(results, fmt.Sprintf("%s:%d:%s", relPath, lineNum, line))
				count++
			}
		}
		return nil
	})

	if err != nil {
		return tool.Result{Success: false, Message: err.Error()}, nil
	}

	output := strings.Join(results, "\n")
	if len(results) == 0 {
		output = "No matches found."
	}

	return tool.Result{
		Success: true,
		Output:  output,
		Metadata: map[string]any{
			"match_count": len(results),
		},
	}, nil
}

func matchesGlob(pattern, relPath string) bool {
	base := filepath.Base(relPath)
	// Check against base name
	if matched, err := filepath.Match(pattern, base); err == nil && matched {
		return true
	}
	// Check against relative path
	if matched, err := filepath.Match(pattern, relPath); err == nil && matched {
		return true
	}
	// Support simple substring globbing (e.g. *.go matching anywhere)
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 && parts[0] == "" && strings.HasSuffix(relPath, parts[1]) {
			return true
		}
	}
	return false
}
