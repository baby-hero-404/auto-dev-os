package orchestrator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestStepRequiresEditCaps(t *testing.T) {
	cases := []struct {
		stepID string
		want   bool
	}{
		{"fix", true},
		{"code_backend", true},
		{"code_backend_0", true},
		{"code_frontend", true},
		{"code_frontend_1", true},
		{"review", false},
		{"analyze", false},
		{"plan", false},
		{"context_load", false},
		{"test", false},
		{"merge", false},
		{"pr", false},
	}
	for _, tc := range cases {
		t.Run(tc.stepID, func(t *testing.T) {
			if got := stepRequiresEditCaps(tc.stepID); got != tc.want {
				t.Errorf("stepRequiresEditCaps(%q) = %v, want %v", tc.stepID, got, tc.want)
			}
		})
	}
}

func TestEffectiveRoleForStep(t *testing.T) {
	// Table-driven tests matching design.md § Role Resolution Matrix
	cases := []struct {
		name            string
		stepID          string
		agentRole       string
		primaryCategory string
		wantRole        string
	}{
		// Edit steps, coder roles -> unchanged
		{"edit step under backend", "fix", "backend", "backend", "backend"},
		{"edit step under frontend", "code_frontend", "frontend", "frontend", "frontend"},

		// Edit steps, non-coder roles -> remap to coder role
		{"edit step under reviewer, primary backend", "fix", "reviewer", "backend", "backend"},
		{"edit step under reviewer, primary frontend", "fix", "reviewer", "frontend", "frontend"},
		{"edit step under reviewer, primary ui", "fix", "reviewer", "ui", "frontend"},
		{"edit step under reviewer, primary ux", "fix", "reviewer", "ux", "frontend"},
		{"edit step under reviewer, empty analysis", "fix", "reviewer", "", "backend"},

		{"edit step under planner", "code_backend", "planner", "backend", "backend"},
		{"edit step under qa", "code_backend", "qa", "backend", "backend"},
		{"edit step under unknown role", "code_backend", "unknown_role", "backend", "backend"},

		// Read-only steps -> unchanged
		{"read-only step review under reviewer", "review", "reviewer", "backend", "reviewer"},
		{"read-only step review under planner", "review", "planner", "backend", "planner"},
		{"read-only step analyze under planner", "analyze", "planner", "backend", "planner"},
		{"read-only step plan under planner", "plan", "planner", "backend", "planner"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var analysis models.TaskAnalysis
			analysis.PrimaryCategory = tc.primaryCategory
			analysisBytes, _ := json.Marshal(analysis)
			task := &models.Task{
				Analysis: analysisBytes,
			}
			gotRole := effectiveRoleForStep(tc.stepID, tc.agentRole, task)
			if gotRole != tc.wantRole {
				t.Errorf("effectiveRoleForStep(%q, %q, Category=%q) = %q, want %q", tc.stepID, tc.agentRole, tc.primaryCategory, gotRole, tc.wantRole)
			}
		})
	}
}

func TestRegression_8291a25e(t *testing.T) {
	// 1. Task analysis is backend or empty -> coderRoleForTask is backend
	analysisJSON, _ := json.Marshal(models.TaskAnalysis{
		PrimaryCategory: "backend",
		ExecutionBoundaries: []models.ExecutionBoundary{
			{Module: "sync", Root: "cmd/sync", Capabilities: []string{"create_helper"}},
		},
	})
	task := &models.Task{
		ID:       "task-reg-1.1",
		Analysis: analysisJSON,
	}

	// Agent is reviewer
	agent := &models.Agent{
		ID:   "agent-reviewer",
		Role: "reviewer",
	}

	// Resolved role for fix step should be backend
	resolvedRole := effectiveRoleForStep("fix", agent.Role, task)
	if resolvedRole != "backend" {
		t.Fatalf("expected resolved role for fix step under reviewer to be 'backend', got %q", resolvedRole)
	}

	// 2. Advertised tools for resolved role should include search_replace and create_file
	reg := tool.NewRegistry()
	// Register the real tools or mock tools
	reg.Register(&mockTool{name: "search_replace", caps: []tool.Capability{tool.CapEdit}})
	reg.Register(&mockTool{name: "create_file", caps: []tool.Capability{tool.CapCreate}})

	cm := tool.NewCapabilityManager(reg, tool.DefaultRoleProfiles())
	tools := cm.ToolsForRole(resolvedRole)

	hasSearchReplace := false
	hasCreateFile := false
	for _, toolDef := range tools {
		if toolDef.Name == "search_replace" {
			hasSearchReplace = true
		}
		if toolDef.Name == "create_file" {
			hasCreateFile = true
		}
	}
	if !hasSearchReplace {
		t.Errorf("expected backend tools to include 'search_replace'")
	}
	if !hasCreateFile {
		t.Errorf("expected backend tools to include 'create_file'")
	}

	// 3. Executing create_file through boundary checked executor succeeds authorization
	mockTasks := &mockTaskRepoRegression{task: task}
	executor := steps.NewBoundaryCheckedToolExecutor(reg, "/workspace", task, agent.ID, resolvedRole, mockTasks)

	args, _ := json.Marshal(map[string]any{"path": "cmd/sync/main.go"})
	res, err := executor(context.Background(), "create_file", string(args))
	if err != nil {
		t.Fatalf("unexpected error executing create_file through boundary executor: %v", err)
	}
	if res != "ok" {
		t.Errorf("expected tool execution to succeed, got result: %q", res)
	}
}

type mockTool struct {
	name string
	caps []tool.Capability
}

func (t *mockTool) Name() string            { return t.name }
func (t *mockTool) Description() string     { return "mock" }
func (t *mockTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }
func (t *mockTool) Category() tool.Category { return tool.CategoryEditing }
func (t *mockTool) Capabilities() []tool.Capability {
	return t.caps
}
func (t *mockTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	return tool.Result{Success: true, Output: "ok"}, nil
}

type mockTaskRepoRegression struct {
	task *models.Task
}

func (m *mockTaskRepoRegression) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return m.task, nil
}

func (m *mockTaskRepoRegression) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	return m.task, nil
}
