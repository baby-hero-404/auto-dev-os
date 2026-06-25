package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

func BuiltinToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		toolDefinition("read_file", "Read a workspace file.", `{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"max_bytes":{"type":"integer","default":12000}}}`),
		toolDefinition("write_file", "Write content to a workspace file.", `{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"content":{"type":"string"}}}`),
		toolDefinition("run_tests", "Run project tests inside the sandbox.", `{"type":"object","properties":{"command":{"type":"string","default":"go test ./..."}}}`),
		toolDefinition("analyze_logs", "Read and summarize a local log file.", `{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"max_bytes":{"type":"integer","default":4000}}}`),
		toolDefinition("generate_docs", "Generate documentation text from a topic and source summary.", `{"type":"object","required":["topic"],"properties":{"topic":{"type":"string"},"summary":{"type":"string"}}}`),
		toolDefinition("create_migration", "Create SQL migration draft content.", `{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"up":{"type":"string"},"down":{"type":"string"}}}`),
		toolDefinition("search_code", "Search source files for a literal string.", `{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"limit":{"type":"integer","default":20}}}`),
		toolDefinition("apply_patch", "Apply a token-efficient search-and-replace edit to a workspace file.", `{"type":"object","required":["path","search","replace"],"properties":{"path":{"type":"string"},"search":{"type":"string"},"replace":{"type":"string"}}}`),
	}
}

func toolDefinition(name, description, schema string) llm.ToolDefinition {
	return llm.ToolDefinition{Name: name, Description: description, Parameters: json.RawMessage(schema)}
}

func (e *SkillExecutor) readFile(call SkillCall) SkillResult {
	target, err := safeWorkspacePath(call.Workspace, stringInput(call.Input, "path", ""))
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	maxBytes := intInput(call.Input, "max_bytes", 12000)
	data, err := os.ReadFile(target)
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	return SkillResult{Name: call.Name, Success: true, Output: string(data)}
}

func (e *SkillExecutor) writeFile(call SkillCall) SkillResult {
	target, err := safeWorkspacePath(call.Workspace, stringInput(call.Input, "path", ""))
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	content := stringInput(call.Input, "content", "")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return skillError(call.Name, err.Error())
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return skillError(call.Name, err.Error())
	}
	return SkillResult{Name: call.Name, Success: true, Output: fmt.Sprintf("wrote %s", stringInput(call.Input, "path", ""))}
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
