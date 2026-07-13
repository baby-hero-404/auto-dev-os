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

func (m *mockTool) Name() string               { return m.name }
func (m *mockTool) Description() string        { return m.desc }
func (m *mockTool) Schema() json.RawMessage    { return m.schema }
func (m *mockTool) Category() Category         { return m.cat }
func (m *mockTool) Capabilities() []Capability { return m.caps }
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

// TestRegistryExecute_RejectsUnauthorizedRole confirms a reviewer-role call to an edit tool
// (search_replace's capability) is rejected by Registry.Execute itself — before the tool's
// Execute (and therefore any filesystem mutation) ever runs — rather than relying solely on
// the diff-path boundary check in boundary_tool_executor.go.
func TestRegistryExecute_RejectsUnauthorizedRole(t *testing.T) {
	r := NewRegistry()
	invoked := false
	searchReplace := &mockTool{
		name: "search_replace",
		caps: []Capability{CapEdit},
		execute: func(ctx context.Context, call Call) (Result, error) {
			invoked = true
			return Result{Success: true, Output: "mutated"}, nil
		},
	}
	r.Register(searchReplace)

	res, err := r.Execute(context.Background(), "search_replace", Call{AgentRole: "reviewer"})
	if err != nil {
		t.Fatalf("expected a clean rejection Result, not a raw error, got: %v", err)
	}
	if res.Success {
		t.Error("expected rejection for reviewer role calling an edit tool")
	}
	if res.Message == "" {
		t.Error("expected a rejection message explaining the authorization failure")
	} else if !strings.HasPrefix(res.Message, "Error: ") {
		t.Errorf("expected rejection message to start with 'Error: ', got: %q", res.Message)
	}
	if invoked {
		t.Error("expected the underlying tool to NOT be invoked when the role is unauthorized")
	}
}

// TestRegistryExecute_AllowsAuthorizedRoleCombinations confirms existing legitimate role/tool
// pairings keep working unchanged after execution-time capability enforcement is added.
func TestRegistryExecute_AllowsAuthorizedRoleCombinations(t *testing.T) {
	cases := []struct {
		name string
		role string
		caps []Capability
	}{
		{"backend -> edit tool", "backend", []Capability{CapEdit}},
		{"reviewer -> read tool", "reviewer", []Capability{CapRead}},
		{"reviewer -> search tool", "reviewer", []Capability{CapSearch}},
		{"reviewer -> git.diff tool", "reviewer", []Capability{CapGit, CapGitDiff}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRegistry()
			invoked := false
			r.Register(&mockTool{
				name: "target_tool",
				caps: tc.caps,
				execute: func(ctx context.Context, call Call) (Result, error) {
					invoked = true
					return Result{Success: true, Output: "ok"}, nil
				},
			})

			res, err := r.Execute(context.Background(), "target_tool", Call{AgentRole: tc.role})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.Success {
				t.Errorf("expected success for authorized role/tool combination, got message: %q", res.Message)
			}
			if !invoked {
				t.Error("expected the underlying tool to be invoked for an authorized role")
			}
		})
	}
}
