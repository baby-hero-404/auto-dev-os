package steps

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// TestNewRegistryToolExecutor_NoDoubleErrorPrefix regression-tests the "Error: Error: role ..."
// double prefix quoted in the task 8291a25e trace report as evidence of the bug under
// investigation: Registry.Execute's authorization rejection already returns a Message
// starting with "Error: ", and the executor used to unconditionally prepend another one.
func TestNewRegistryToolExecutor_NoDoubleErrorPrefix(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(&recordingTool{name: "create_file", caps: []tool.Capability{tool.CapCreate}})

	executor := NewRegistryToolExecutor(registry, "/workspace", "task-1", "agent-1", "reviewer")

	args, _ := json.Marshal(map[string]any{"path": "cmd/main.go"})
	result, err := executor(context.Background(), "create_file", string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "Error: Error:") {
		t.Errorf("expected no double 'Error: Error:' prefix, got: %q", result)
	}
	if !strings.HasPrefix(result, "Error: role ") {
		t.Errorf("expected a single 'Error: role ...' prefix, got: %q", result)
	}
}
