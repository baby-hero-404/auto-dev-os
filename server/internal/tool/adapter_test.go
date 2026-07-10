package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/skills"
)

type dummyTool struct{}

func (t *dummyTool) Name() string             { return "dummy" }
func (t *dummyTool) Description() string      { return "dummy tool" }
func (t *dummyTool) Schema() json.RawMessage  { return json.RawMessage(`{}`) }
func (t *dummyTool) Category() Category       { return CategorySearch }
func (t *dummyTool) Capabilities() []Capability { return []Capability{CapSearch} }
func (t *dummyTool) Execute(ctx context.Context, call Call) (Result, error) {
	val, _ := call.Input["foo"].(string)
	return Result{
		Success: true,
		Output:  "hello " + val,
	}, nil
}

func TestSkillExecutorAdapter(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{})

	adapter := NewSkillExecutorAdapter(reg)

	// Test 1: Successful execution
	res := adapter.Execute(context.Background(), skills.SkillCall{
		Name:      "dummy",
		Input:     map[string]any{"foo": "world"},
		Workspace: "/tmp",
	})

	if !res.Success {
		t.Errorf("expected success, got error: %s", res.Error)
	}
	if res.Output != "hello world" {
		t.Errorf("expected output 'hello world', got %q", res.Output)
	}

	// Test 2: Unknown tool
	resUnknown := adapter.Execute(context.Background(), skills.SkillCall{
		Name:      "nonexistent",
		Workspace: "/tmp",
	})
	if resUnknown.Success {
		t.Errorf("expected unknown tool execution to fail")
	}
	if resUnknown.Error != "unknown tool: nonexistent" {
		t.Errorf("expected not found error message, got %q", resUnknown.Error)
	}
}
