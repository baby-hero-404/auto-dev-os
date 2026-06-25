package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

type SkillCall struct {
	Name         string         `json:"name"`
	Input        map[string]any `json:"input"`
	Workspace    string         `json:"workspace"`
	TaskID       string         `json:"task_id"`
	AgentID      string         `json:"agent_id"`
	AgentName    string         `json:"agent_name,omitempty"`
	AllowedTools []string       `json:"allowed_tools,omitempty"`
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

func (e *SkillExecutor) Execute(ctx context.Context, call SkillCall) SkillResult {
	if !callAllowsTool(call) {
		agentName := call.AgentName
		if agentName == "" {
			agentName = call.AgentID
		}
		return skillError(call.Name, fmt.Sprintf("agent %s is not authorized to use tool %s", agentName, call.Name))
	}
	if call.Workspace == "" {
		return skillError(call.Name, "workspace is required")
	}
	switch call.Name {
	case "read_file":
		return e.readFile(call)
	case "write_file":
		return e.writeFile(call)
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

func callAllowsTool(call SkillCall) bool {
	if len(call.AllowedTools) == 0 {
		return true
	}
	for _, tool := range call.AllowedTools {
		if strings.EqualFold(strings.TrimSpace(tool), call.Name) {
			return true
		}
	}
	return false
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
