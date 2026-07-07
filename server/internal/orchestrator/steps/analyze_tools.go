package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func analyzeToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "list_files",
			Description: "List relevant source files in the task workspace. Use this before reading files when repository structure is unknown.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "read_file",
			Description: "Read a single workspace file by repository-relative path.",
			Parameters:  json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string","description":"Repository-relative file path to read."}}}`),
		},
		{
			Name:        "grep_search",
			Description: "Search workspace files for a literal query string and return matching lines.",
			Parameters:  json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string","description":"Literal text to search for."}}}`),
		},
	}
}

func (s *AnalyzeStep) executeAnalyzeTool(ctx context.Context, toolName, arguments string) string {
	var args map[string]any
	if strings.TrimSpace(arguments) != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return fmt.Sprintf("Error: invalid tool arguments JSON: %v", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	switch toolName {
	case "list_files":
		result, err := s.listAnalyzeFiles(ctx)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	case "read_file":
		path, _ := args["path"].(string)
		if strings.TrimSpace(path) == "" {
			return `Error: missing required "path" argument`
		}
		result, err := s.readAnalyzeFile(ctx, path)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	case "grep_search":
		query, _ := args["query"].(string)
		if strings.TrimSpace(query) == "" {
			return `Error: missing required "query" argument`
		}
		result, err := s.grepAnalyzeFiles(ctx, query)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	default:
		return fmt.Sprintf("Error: unknown analyze tool %q", toolName)
	}
}

type analyzeSourceRoot struct {
	path   string
	prefix string
}

func (s *AnalyzeStep) analyzeSourceRoots(ctx context.Context) []analyzeSourceRoot {
	localPath := sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID)
	if s.wkspace == nil {
		return []analyzeSourceRoot{{path: localPath}}
	}
	ws, err := s.wkspace.LoadTaskWorkspace(ctx, s.rt.Task)
	if err != nil || ws == nil || len(ws.Repos) == 0 {
		return []analyzeSourceRoot{{path: localPath}}
	}

	var roots []analyzeSourceRoot
	targetCount := 0
	for _, repo := range ws.Repos {
		if s.rt.Task.RepositoryID != nil && repo.RepoID != *s.rt.Task.RepositoryID {
			continue
		}
		if repo.Paths.Main == "" {
			continue
		}
		targetCount++
	}
	for _, repo := range ws.Repos {
		if s.rt.Task.RepositoryID != nil && repo.RepoID != *s.rt.Task.RepositoryID {
			continue
		}
		if repo.Paths.Main == "" {
			continue
		}
		prefix := ""
		if s.rt.Task.RepositoryID == nil && targetCount > 1 {
			prefix = repo.Name
		}
		roots = append(roots, analyzeSourceRoot{
			path:   filepath.Join(ws.Root, repo.Paths.Main),
			prefix: prefix,
		})
	}
	if len(roots) == 0 {
		return []analyzeSourceRoot{{path: localPath}}
	}
	return roots
}

func (s *AnalyzeStep) getContainerRoot(rootPath string) string {
	if s.containerPath != nil {
		return s.containerPath(s.rt.Task, rootPath, "")
	}
	localPath := sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID)
	rel, err := filepath.Rel(localPath, rootPath)
	if err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return "/workspace"
		}
		return "/workspace/" + rel
	}
	return "/workspace"
}

func (s *AnalyzeStep) listAnalyzeFiles(ctx context.Context) (string, error) {
	var files []string
	for _, root := range s.analyzeSourceRoots(ctx) {
		containerRoot := s.getContainerRoot(root.path)
		cmd := fmt.Sprintf("cd %s && find . \\( -name .git -o -name node_modules -o -name vendor -o -name dist -o -name artifacts -o -name logs -o -name specs -o -name openspec -o -name context -o -name pr \\) -prune -o -type f -print | sed 's#^\\\\./##'", paths.QuoteShellArg(containerRoot))
		out, err := s.runAnalyzeSandboxCommand(ctx, cmd)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			files = append(files, filepath.ToSlash(filepath.Join(root.prefix, line)))
		}
	}
	if len(files) == 0 {
		return "No files found in workspace.", nil
	}
	return strings.Join(files, "\n"), nil
}

func (s *AnalyzeStep) readAnalyzeFile(ctx context.Context, subPath string) (string, error) {
	subPath = filepath.Clean(strings.TrimSpace(subPath))
	for _, root := range s.analyzeSourceRoots(ctx) {
		relPath := subPath
		if root.prefix != "" {
			prefix := root.prefix + string(filepath.Separator)
			if !strings.HasPrefix(subPath, prefix) {
				continue
			}
			relPath = strings.TrimPrefix(subPath, prefix)
		}
		if !paths.IsSafeRelativeSourcePath(relPath) {
			continue
		}
		containerRoot := s.getContainerRoot(root.path)
		cmd := fmt.Sprintf("cd %s && if [ -f %s ]; then head -c 20000 %s; else exit 2; fi",
			paths.QuoteShellArg(containerRoot),
			paths.QuoteShellArg(relPath),
			paths.QuoteShellArg(relPath),
		)
		content, err := s.runAnalyzeSandboxCommand(ctx, cmd)
		if err == nil {
			return content, nil
		}
	}
	return "", fmt.Errorf("file %s not found in source roots", subPath)
}

func (s *AnalyzeStep) grepAnalyzeFiles(ctx context.Context, query string) (string, error) {
	var matches []string
	for _, root := range s.analyzeSourceRoots(ctx) {
		containerRoot := s.getContainerRoot(root.path)
		cmd := fmt.Sprintf("cd %s && grep -RIn --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=vendor --exclude-dir=dist --exclude-dir=artifacts --exclude-dir=logs --exclude-dir=specs --exclude-dir=openspec --exclude-dir=context --exclude-dir=pr -- %s . || true",
			paths.QuoteShellArg(containerRoot),
			paths.QuoteShellArg(query),
		)
		result, err := s.runAnalyzeSandboxCommand(ctx, cmd)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(result, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if after, ok := strings.CutPrefix(line, "./"); ok {
				line = after
			}
			matches = append(matches, filepath.ToSlash(filepath.Join(root.prefix, line)))
		}
	}
	if len(matches) == 0 {
		return "No matches found.", nil
	}
	return strings.Join(matches, "\n"), nil
}

func (s *AnalyzeStep) runAnalyzeSandboxCommand(ctx context.Context, command string) (string, error) {
	if s.sandbox == nil {
		return "", fmt.Errorf("sandbox runner is not configured")
	}
	result, err := s.sandbox.RunCommand(ctx, s.rt.Task, s.rt.Agent, "analyze_sandbox_cmd", command)
	if err != nil {
		return "", err
	}
	exitCode, _ := result["exit_code"].(int)
	stdout, _ := result["stdout"].(string)
	stderr, _ := result["stderr"].(string)
	if exitCode != 0 {
		return "", fmt.Errorf("analyze sandbox command failed with exit status %d: %s", exitCode, strings.TrimSpace(stderr))
	}
	return stdout, nil
}
