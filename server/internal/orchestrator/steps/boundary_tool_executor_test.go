package steps

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// recordingTool is a minimal tool.Tool that records whether it was invoked, for asserting
// that the boundary check does (or doesn't) let a call reach the underlying registry.
type recordingTool struct {
	name    string
	caps    []tool.Capability
	invoked bool
}

func (t *recordingTool) Name() string            { return t.name }
func (t *recordingTool) Description() string     { return "test tool" }
func (t *recordingTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }
func (t *recordingTool) Category() tool.Category { return tool.CategoryEditing }
func (t *recordingTool) Capabilities() []tool.Capability {
	if t.caps != nil {
		return t.caps
	}
	return []tool.Capability{tool.CapEdit}
}
func (t *recordingTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	t.invoked = true
	return tool.Result{Success: true, Output: "ok"}, nil
}

func newBoundaryTestRegistry(searchReplace, readFile *recordingTool) *tool.Registry {
	r := tool.NewRegistry()
	r.Register(searchReplace)
	r.Register(readFile)
	return r
}

func TestNewBoundaryCheckedToolExecutor_BlocksCriticalPathWithPause(t *testing.T) {
	searchReplace := &recordingTool{name: "search_replace"}
	readFile := &recordingTool{name: "read_file", caps: []tool.Capability{tool.CapRead}}
	registry := newBoundaryTestRegistry(searchReplace, readFile)

	task := &models.Task{ID: "task-1"}
	tasks := &mockTaskReader{task: task}

	executor := NewBoundaryCheckedToolExecutor(registry, "/workspace", task, "agent-1", "backend", tasks)

	args, _ := json.Marshal(map[string]any{"path": ".github/workflows/ci.yml"})
	_, err := executor(context.Background(), "search_replace", string(args))
	if err == nil {
		t.Fatal("expected a critical boundary violation to return an error")
	}
	if !errors.Is(err, workflow.ErrPaused) {
		t.Errorf("expected error to wrap workflow.ErrPaused, got: %v", err)
	}
	if searchReplace.invoked {
		t.Error("expected the underlying tool to NOT be invoked for a critical violation")
	}
}

func TestNewBoundaryCheckedToolExecutor_AllowsInBoundaryEdit(t *testing.T) {
	searchReplace := &recordingTool{name: "search_replace"}
	readFile := &recordingTool{name: "read_file", caps: []tool.Capability{tool.CapRead}}
	registry := newBoundaryTestRegistry(searchReplace, readFile)

	analysisJSON, _ := json.Marshal(models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{Module: "service", Root: "internal/service", Capabilities: []string{"modify_existing"}},
		},
	})
	task := &models.Task{ID: "task-2", Analysis: analysisJSON}
	tasks := &mockTaskReader{task: task}

	executor := NewBoundaryCheckedToolExecutor(registry, "/workspace", task, "agent-1", "backend", tasks)

	args, _ := json.Marshal(map[string]any{"path": "internal/service/handler.go"})
	result, err := executor(context.Background(), "search_replace", string(args))
	if err != nil {
		t.Fatalf("unexpected hard error for an ordinary in-repo edit: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected the underlying tool result to pass through, got: %q", result)
	}
	if !searchReplace.invoked {
		t.Error("expected the underlying search_replace tool to be invoked")
	}
}

func TestNewBoundaryCheckedToolExecutor_OutsideBoundaryReturnsSoftError(t *testing.T) {
	searchReplace := &recordingTool{name: "search_replace"}
	readFile := &recordingTool{name: "read_file", caps: []tool.Capability{tool.CapRead}}
	registry := newBoundaryTestRegistry(searchReplace, readFile)

	analysisJSON, _ := json.Marshal(models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{Module: "service", Root: "internal/service", Capabilities: []string{"modify_existing"}},
		},
	})
	task := &models.Task{ID: "task-4", Analysis: analysisJSON}
	tasks := &mockTaskReader{task: task}

	executor := NewBoundaryCheckedToolExecutor(registry, "/workspace", task, "agent-1", "backend", tasks)

	// A file outside every declared execution boundary should be a soft (non-pausing) error
	// fed back to the LLM, not a hard abort — matching SeverityError handling in the
	// diff-based path today.
	args, _ := json.Marshal(map[string]any{"path": "internal/other/unrelated.go"})
	result, err := executor(context.Background(), "search_replace", string(args))
	if err != nil {
		t.Fatalf("expected a soft error string, not a hard error, got: %v", err)
	}
	if result == "ok" {
		t.Error("expected an execution boundary violation message, not the tool's success output")
	}
	if searchReplace.invoked {
		t.Error("expected the underlying tool to NOT be invoked for an out-of-boundary edit")
	}
}

func TestNewBoundaryCheckedToolExecutor_ReadOnlyToolsBypassBoundaryCheck(t *testing.T) {
	searchReplace := &recordingTool{name: "search_replace"}
	readFile := &recordingTool{name: "read_file", caps: []tool.Capability{tool.CapRead}}
	registry := newBoundaryTestRegistry(searchReplace, readFile)

	task := &models.Task{ID: "task-3"}
	tasks := &mockTaskReader{task: task}

	executor := NewBoundaryCheckedToolExecutor(registry, "/workspace", task, "agent-1", "reviewer", tasks)

	// Even a "critical" path should pass straight through for non-edit tools like read_file.
	args, _ := json.Marshal(map[string]any{"path": ".github/workflows/ci.yml"})
	result, err := executor(context.Background(), "read_file", string(args))
	if err != nil {
		t.Fatalf("unexpected error for a read-only tool call: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected the underlying tool result to pass through, got: %q", result)
	}
	if !readFile.invoked {
		t.Error("expected the underlying read_file tool to be invoked")
	}
}
