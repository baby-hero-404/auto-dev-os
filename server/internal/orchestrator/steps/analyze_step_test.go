package steps

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/tool/tools"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockPromptAssembler struct {
	messages []llm.Message
	tools    []llm.ToolDefinition
	err      error
}

func (m *mockPromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message, tools []llm.ToolDefinition) ([]llm.Message, []llm.ToolDefinition, error) {
	return m.messages, m.tools, m.err
}

func (m *mockPromptAssembler) ListAllSkills(ctx context.Context, task models.Task) ([]llm.ToolDefinition, error) {
	return m.tools, m.err
}

type mockTraceRecorder struct {
	called bool
}

func (m *mockTraceRecorder) WriteLLMCallTrace(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed StepResult, retryAttempt int, latency time.Duration) {
	m.called = true
}

func TestAnalyzeStep_SkipsWhenReady(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-step-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:         "task-123",
		ProjectID:  "proj-123",
		Complexity: models.TaskComplexityEasy,
		SpecStatus: models.TaskSpecStatusAutoApproved,
	}

	step := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
		&mockProjectReader{},
		&mockLLMChatter{},
		&mockPromptAssembler{},
		&mockSandboxRunner{},
		&mockArtifactSaver{},
		&mockStatusUpdater{},
		&mockTraceRecorder{},
		&mockLogger{},
		nil, // wkspace
		nil, // containerPath
		8.0, // maxCost
		tools.DefaultRegistry(nil, nil),
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["complexity"] != models.TaskComplexityEasy {
		t.Errorf("expected complexity easy, got: %v", result["complexity"])
	}
	if result["spec_status"] != models.TaskSpecStatusAutoApproved {
		t.Errorf("expected spec status auto_approved, got: %v", result["spec_status"])
	}
}

func TestAnalyzeStep_RunsAnalysisAutoApprove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-step-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:         "task-123",
		ProjectID:  "proj-123",
		Complexity: "easy",
		Status:     models.TaskStatusTodo,
	}

	llmResponse := `
{
  "complexity": "easy",
  "primary_category": "backend",
  "scope": "Implement basic sum function",
  "affected_files": ["math.go"],
  "risks": [],
  "risk_domains": [],
  "execution_phases": [
    {
      "phase": "Phase 1: Setup",
      "tasks": ["write code"]
    }
  ],
  "clarification_questions": [],
  "required_skills": [],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": {"allowed": ["."]},
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	chatter := &mockLLMChatter{
		resp: &llm.Response{
			Content: llmResponse,
			Model:   "test-model",
		},
	}

	taskUpdate := &mockTaskReader{task: task} // stub task reader/updater
	statusMock := &mockStatusUpdater{}
	traceMock := &mockTraceRecorder{}

	step := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", AutonomyLevel: "high"}, JobID: "j1"},
		tmpDir,
		taskUpdate,
		nil, // taskUpdater not needed if nil update check is safe
		&mockProjectReader{project: &models.Project{DefaultAutonomy: "high"}},
		chatter,
		&mockPromptAssembler{},
		&mockSandboxRunner{},
		&mockArtifactSaver{},
		statusMock,
		traceMock,
		&mockLogger{},
		nil, // wkspace
		nil, // containerPath
		8.0, // maxCost
		tools.DefaultRegistry(nil, nil),
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["complexity"] != "easy" {
		t.Errorf("expected complexity easy, got: %v", result["complexity"])
	}

	if !statusMock.called {
		t.Error("expected status updater to be called")
	}

	var analysis models.TaskAnalysis
	if errUnmarshal := json.Unmarshal(task.Analysis, &analysis); errUnmarshal != nil {
		t.Fatalf("failed to unmarshal saved analysis: %v", errUnmarshal)
	}
	if analysis.PrimaryCategory != "backend" {
		t.Errorf("expected category backend, got: %s", analysis.PrimaryCategory)
	}
}

func TestNormalizeTaskID(t *testing.T) {
	tests := []struct {
		input         string
		fallbackPhase int
		fallbackTask  int
		expected      string
	}{
		{"Khởi tạo dự án", 1, 1, "Task 1.1: Khởi tạo dự án"},
		{"task 1.2: Định nghĩa interface", 2, 3, "Task 1.2: Định nghĩa interface"},
		{"Task-2.1 - Triển khai SQLite", 3, 4, "Task 2.1: Triển khai SQLite"},
		{"Task 3.2: Sync engine", 0, 0, "Task 3.2: Sync engine"},
		{"Triển khai scheduler", 0, 0, "Triển khai scheduler"},
	}

	for _, tt := range tests {
		actual := normalizeTaskID(tt.input, tt.fallbackPhase, tt.fallbackTask)
		if actual != tt.expected {
			t.Errorf("normalizeTaskID(%q, %d, %d) = %q; expected %q", tt.input, tt.fallbackPhase, tt.fallbackTask, actual, tt.expected)
		}
	}
}
