package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	orchestratorworkspace "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func analyzeToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "list_files",
			Description: "List relevant source files in the task workspace. Use this before reading files when repository structure is unknown.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
		{
			Name:        "read_file",
			Description: "Read a single workspace file by repository-relative path.",
			Parameters:  json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string","description":"Repository-relative file path to read."}},"additionalProperties":false}`),
		},
		{
			Name:        "grep_search",
			Description: "Search workspace files for a literal query string and return matching lines.",
			Parameters:  json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string","description":"Literal text to search for."}},"additionalProperties":false}`),
		},
	}
}

func executeAnalyzeTool(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, toolName, arguments string) string {
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
		result, err := listAnalyzeFiles(ctx, deps, task, agent)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	case "read_file":
		path, _ := args["path"].(string)
		if strings.TrimSpace(path) == "" {
			return `Error: missing required "path" argument`
		}
		result, err := readAnalyzeFile(ctx, deps, task, agent, path)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	case "grep_search":
		query, _ := args["query"].(string)
		if strings.TrimSpace(query) == "" {
			return `Error: missing required "query" argument`
		}
		result, err := grepAnalyzeFiles(ctx, deps, task, agent, query)
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

func analyzeSourceRoots(ctx context.Context, deps *Deps, task *models.Task) []analyzeSourceRoot {
	localPath := sandbox.WorkspacePath(deps.WorkspaceRoot, task.ID)
	if deps.Wkspace == nil {
		return []analyzeSourceRoot{{path: localPath}}
	}
	ws, err := deps.Wkspace.LoadTaskWorkspace(ctx, task)
	if err != nil || ws == nil || len(ws.Repos) == 0 {
		return []analyzeSourceRoot{{path: localPath}}
	}

	var roots []analyzeSourceRoot
	targetCount := 0
	for _, repo := range ws.Repos {
		if task.RepositoryID != nil && repo.RepoID != *task.RepositoryID {
			continue
		}
		if repo.Paths.Main == "" {
			continue
		}
		targetCount++
	}
	for _, repo := range ws.Repos {
		if task.RepositoryID != nil && repo.RepoID != *task.RepositoryID {
			continue
		}
		if repo.Paths.Main == "" {
			continue
		}
		prefix := ""
		if task.RepositoryID == nil && targetCount > 1 {
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

func listAnalyzeFiles(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent) (string, error) {
	var files []string
	for _, root := range analyzeSourceRoots(ctx, deps, task) {
		containerRoot := deps.ContainerPathForHostPath(task, root.path, "")
		cmd := fmt.Sprintf("cd %s && find . \\( -name .git -o -name node_modules -o -name vendor -o -name dist -o -name artifacts -o -name logs -o -name specs -o -name openspec -o -name context -o -name pr \\) -prune -o -type f -print | sed 's#^\\\\./##'", orchestratorworkspace.QuoteShellArg(containerRoot))
		out, err := runAnalyzeSandboxCommand(ctx, deps, task, agent, cmd)
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

func readAnalyzeFile(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, subPath string) (string, error) {
	subPath = filepath.Clean(strings.TrimSpace(subPath))
	for _, root := range analyzeSourceRoots(ctx, deps, task) {
		relPath := subPath
		if root.prefix != "" {
			prefix := root.prefix + string(filepath.Separator)
			if !strings.HasPrefix(subPath, prefix) {
				continue
			}
			relPath = strings.TrimPrefix(subPath, prefix)
		}
		if !orchestratorworkspace.IsSafeRelativeSourcePath(relPath) {
			continue
		}
		containerRoot := deps.ContainerPathForHostPath(task, root.path, "")
		cmd := fmt.Sprintf("cd %s && if [ -f %s ]; then head -c 20000 %s; else exit 2; fi",
			orchestratorworkspace.QuoteShellArg(containerRoot),
			orchestratorworkspace.QuoteShellArg(relPath),
			orchestratorworkspace.QuoteShellArg(relPath),
		)
		content, err := runAnalyzeSandboxCommand(ctx, deps, task, agent, cmd)
		if err == nil {
			return content, nil
		}
	}
	return "", fmt.Errorf("file %s not found in source roots", subPath)
}

func grepAnalyzeFiles(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, query string) (string, error) {
	var matches []string
	for _, root := range analyzeSourceRoots(ctx, deps, task) {
		containerRoot := deps.ContainerPathForHostPath(task, root.path, "")
		cmd := fmt.Sprintf("cd %s && grep -RIn --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=vendor --exclude-dir=dist --exclude-dir=artifacts --exclude-dir=logs --exclude-dir=specs --exclude-dir=openspec --exclude-dir=context --exclude-dir=pr -- %s . || true",
			orchestratorworkspace.QuoteShellArg(containerRoot),
			orchestratorworkspace.QuoteShellArg(query),
		)
		result, err := runAnalyzeSandboxCommand(ctx, deps, task, agent, cmd)
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

func runAnalyzeSandboxCommand(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, command string) (string, error) {
	if deps.Runtime == nil {
		return "", fmt.Errorf("sandbox runtime is not configured")
	}
	agentID := ""
	if agent != nil {
		agentID = agent.ID
	}
	localPath := sandbox.WorkspacePath(deps.WorkspaceRoot, task.ID)
	result, err := deps.Runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agentID,
		Workspace:   localPath,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     time.Minute,
	})
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("analyze sandbox command failed with exit code %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return result.Stdout, nil
}
