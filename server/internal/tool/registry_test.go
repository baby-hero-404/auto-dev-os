package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type mockTool struct {
	name    string
	desc    string
	schema  json.RawMessage
	cat     Category
	caps    []Capability
	execute func(ctx context.Context, call Call) (Result, error)
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return m.desc }
func (m *mockTool) Schema() json.RawMessage       { return m.schema }
func (m *mockTool) Category() Category            { return m.cat }
func (m *mockTool) Capabilities() []Capability    { return m.caps }
func (m *mockTool) Execute(ctx context.Context, call Call) (Result, error) {
	if m.execute != nil {
		return m.execute(ctx, call)
	}
	return Result{Success: true}, nil
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	t1 := &mockTool{name: "tool1", desc: "desc1", schema: json.RawMessage(`{}`), cat: CategoryFilesystem, caps: []Capability{CapRead}}
	t2 := &mockTool{name: "tool2", desc: "desc2", schema: json.RawMessage(`{}`), cat: CategoryEditing, caps: []Capability{CapEdit}}
	t3 := &mockTool{name: "tool3", desc: "desc3", schema: json.RawMessage(`{}`), cat: CategoryGit, caps: []Capability{CapGit}}

	r.Register(t1)
	r.Register(t2)
	r.Register(t3)

	// Panic on duplicate
	defer func() {
		if recover() == nil {
			t.Errorf("Expected Register to panic on duplicate name")
		}
	}()
	r.Register(t1)
}

func TestRegistryDefinitionsAndFiltering(t *testing.T) {
	r := NewRegistry()

	t1 := &mockTool{name: "tool1", desc: "desc1", schema: json.RawMessage(`{}`), cat: CategoryFilesystem, caps: []Capability{CapRead}}
	t2 := &mockTool{name: "tool2", desc: "desc2", schema: json.RawMessage(`{}`), cat: CategoryEditing, caps: []Capability{CapEdit}}
	t3 := &mockTool{name: "tool3", desc: "desc3", schema: json.RawMessage(`{}`), cat: CategoryGit, caps: []Capability{CapGit}}

	r.Register(t1)
	r.Register(t2)
	r.Register(t3)

	defs := r.Definitions()
	if len(defs) != 3 {
		t.Errorf("Expected 3 definitions, got %d", len(defs))
	}

	readTools := r.ToolsForCapabilities([]Capability{CapRead})
	if len(readTools) != 1 || readTools[0].Name != "tool1" {
		t.Errorf("Expected only tool1 for CapRead")
	}

	multipleTools := r.ToolsForCapabilities([]Capability{CapRead, CapEdit})
	if len(multipleTools) != 2 {
		t.Errorf("Expected 2 tools for CapRead and CapEdit, got %d", len(multipleTools))
	}
}

func TestRegistryExecute(t *testing.T) {
	r := NewRegistry()

	t1 := &mockTool{
		name:   "tool1",
		desc:   "desc1",
		schema: json.RawMessage(`{}`),
		cat:    CategoryFilesystem,
		caps:   []Capability{CapRead},
		execute: func(ctx context.Context, call Call) (Result, error) {
			return Result{Success: true, Output: "executed tool1"}, nil
		},
	}
	r.Register(t1)

	// Execute known tool
	res, err := r.Execute(context.Background(), "tool1", Call{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !res.Success || res.Output != "executed tool1" {
		t.Errorf("Execute output mismatch")
	}

	// Execute unknown tool
	_, errUnknown := r.Execute(context.Background(), "unknown_tool", Call{})
	if errUnknown == nil {
		t.Errorf("Expected error for unknown tool")
	}
	if !strings.Contains(errUnknown.Error(), "unknown tool: unknown_tool") {
		t.Errorf("Expected unknown tool error message, got %v", errUnknown)
	}
}
