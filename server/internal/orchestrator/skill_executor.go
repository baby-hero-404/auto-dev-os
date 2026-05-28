package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type SkillCall struct {
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
	Workspace string         `json:"workspace"`
	TaskID    string         `json:"task_id"`
	AgentID   string         `json:"agent_id"`
}

type SkillResult struct {
	Name    string `json:"name"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type SkillExecutor struct {
	runtime sandbox.Runtime
}

func NewSkillExecutor(runtime sandbox.Runtime) *SkillExecutor {
	return &SkillExecutor{runtime: runtime}
}

func BuiltinToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		toolDefinition("run_tests", "Run project tests inside the sandbox.", `{"type":"object","properties":{"command":{"type":"string","default":"go test ./..."}}}`),
		toolDefinition("analyze_logs", "Read and summarize a local log file.", `{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"max_bytes":{"type":"integer","default":4000}}}`),
		toolDefinition("generate_docs", "Generate documentation text from a topic and source summary.", `{"type":"object","required":["topic"],"properties":{"topic":{"type":"string"},"summary":{"type":"string"}}}`),
		toolDefinition("create_migration", "Create SQL migration draft content.", `{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"up":{"type":"string"},"down":{"type":"string"}}}`),
		toolDefinition("search_code", "Search source files for a literal string.", `{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"limit":{"type":"integer","default":20}}}`),
		toolDefinition("apply_patch", "Apply a token-efficient search-and-replace edit to a workspace file.", `{"type":"object","required":["path","search","replace"],"properties":{"path":{"type":"string"},"search":{"type":"string"},"replace":{"type":"string"}}}`),
	}
}

func toolDefinition(name, description, schema string) ToolDefinition {
	return ToolDefinition{Name: name, Description: description, Parameters: json.RawMessage(schema)}
}

func (e *SkillExecutor) Execute(ctx context.Context, call SkillCall) SkillResult {
	if call.Workspace == "" {
		return skillError(call.Name, "workspace is required")
	}
	switch call.Name {
	case "run_tests":
		return e.runSandboxCommand(ctx, call, stringInput(call.Input, "command", "go test ./..."))
	case "analyze_logs":
		return e.analyzeLogs(call)
	case "generate_docs":
		return SkillResult{Name: call.Name, Success: true, Output: generateDocs(call.Input)}
	case "create_migration":
		return SkillResult{Name: call.Name, Success: true, Output: createMigration(call.Input)}
	case "search_code":
		return e.searchCode(call)
	case "apply_patch":
		return e.applySearchReplace(call)
	default:
		return skillError(call.Name, "unknown skill")
	}
}

func (e *SkillExecutor) runSandboxCommand(ctx context.Context, call SkillCall, command string) SkillResult {
	if e.runtime == nil {
		return skillError(call.Name, "sandbox runtime is not configured")
	}
	resp, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      call.TaskID,
		AgentID:     call.AgentID,
		Workspace:   call.Workspace,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
	})
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	output := strings.TrimSpace(resp.Stdout + "\n" + resp.Stderr)
	return SkillResult{Name: call.Name, Success: resp.ExitCode == 0, Output: output}
}

func (e *SkillExecutor) analyzeLogs(call SkillCall) SkillResult {
	target, err := safeWorkspacePath(call.Workspace, stringInput(call.Input, "path", ""))
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	maxBytes := intInput(call.Input, "max_bytes", 4000)
	data, err := os.ReadFile(target)
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return SkillResult{Name: call.Name, Success: true, Output: string(data)}
}

func (e *SkillExecutor) searchCode(call SkillCall) SkillResult {
	query := stringInput(call.Input, "query", "")
	if query == "" {
		return skillError(call.Name, "query is required")
	}
	limit := intInput(call.Input, "limit", 20)
	var matches []string
	err := filepath.WalkDir(call.Workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(matches) >= limit {
			return err
		}
		if shouldSkipSearchPath(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), query) {
			rel, _ := filepath.Rel(call.Workspace, path)
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	return SkillResult{Name: call.Name, Success: true, Output: strings.Join(matches, "\n")}
}

func (e *SkillExecutor) applySearchReplace(call SkillCall) SkillResult {
	target, err := safeWorkspacePath(call.Workspace, stringInput(call.Input, "path", ""))
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	search := stringInput(call.Input, "search", "")
	if search == "" {
		return skillError(call.Name, "search is required")
	}
	replace := stringInput(call.Input, "replace", "")
	data, err := os.ReadFile(target)
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	content := string(data)
	if !strings.Contains(content, search) {
		return skillError(call.Name, "search block not found")
	}
	updated := strings.Replace(content, search, replace, 1)
	if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
		return skillError(call.Name, err.Error())
	}
	return SkillResult{Name: call.Name, Success: true, Output: fmt.Sprintf("updated %s", stringInput(call.Input, "path", ""))}
}

func safeWorkspacePath(workspace, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	cleanWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(cleanWorkspace, rel))
	if !strings.HasPrefix(target, cleanWorkspace+string(os.PathSeparator)) && target != cleanWorkspace {
		return "", fmt.Errorf("path escapes workspace")
	}
	return target, nil
}

func shouldSkipSearchPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		switch part {
		case ".git", "node_modules", "vendor", "dist", "build":
			return true
		}
	}
	return false
}

func generateDocs(input map[string]any) string {
	topic := stringInput(input, "topic", "Untitled")
	summary := stringInput(input, "summary", "")
	if summary == "" {
		return "# " + topic + "\n\nTODO: add implementation notes.\n"
	}
	return "# " + topic + "\n\n" + summary + "\n"
}

func createMigration(input map[string]any) string {
	name := stringInput(input, "name", "migration")
	up := stringInput(input, "up", "-- TODO: add up migration")
	down := stringInput(input, "down", "-- TODO: add down migration")
	return fmt.Sprintf("-- %s.up.sql\n%s\n\n-- %s.down.sql\n%s\n", name, up, name, down)
}

func stringInput(input map[string]any, key, fallback string) string {
	value, ok := input[key]
	if !ok {
		return fallback
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
}

func intInput(input map[string]any, key string, fallback int) int {
	value, ok := input[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return fallback
	}
}

func skillError(name, msg string) SkillResult {
	return SkillResult{Name: name, Success: false, Error: msg}
}
