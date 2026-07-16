package steps

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
  "affected_files": [{"file": "math.go", "confidence": 1.0, "reason": "edit"}],
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
  "required_skills_map": {},
  "execution_units": [],
  "execution_irs": [{"node_id": "n1", "intent": {"capability": "x", "operation": "y"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
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

// sequencedLLMChatter returns a different queued response on each successive call, falling back
// to the last response once the queue is exhausted — used to simulate a model that corrects its
// output after corrective feedback within the same tool loop.
type sequencedLLMChatter struct {
	responses []*llm.Response
	calls     int
}

func (m *sequencedLLMChatter) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (m *sequencedLLMChatter) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.calls++
	return m.responses[idx], nil
}

// TestAnalyzeStep_ContractValidationRetriesThenSucceeds verifies that after migrating onto the
// shared llmrunner.RunToolLoop (Task 4.2 / REQ-M08), the analyze step's execution-contract field
// validation still drives a real retry: a spec JSON missing required fields must be rejected via
// the Validate callback and fed back to the model, and a subsequent complete spec must be
// accepted, exercising the exact mechanism validateAnalyzeSpec/analyzeContractMissingFields
// implement.
func TestAnalyzeStep_ContractValidationRetriesThenSucceeds(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-step-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:         "task-retry",
		ProjectID:  "proj-retry",
		Complexity: "easy",
		Status:     models.TaskStatusTodo,
	}

	incompleteResponse := `{"complexity": "easy"}`
	completeResponse := `
{
  "complexity": "easy",
  "primary_category": "backend",
  "scope": "Implement basic sum function",
  "affected_files": [{"file": "math.go", "confidence": 1.0, "reason": "edit"}],
  "risks": [],
  "risk_domains": [],
  "execution_phases": [{"phase": "Phase 1: Setup", "tasks": ["write code"]}],
  "clarification_questions": [],
  "required_skills": [],
  "required_skills_map": {},
  "execution_units": [],
  "execution_irs": [{"node_id": "n1", "intent": {"capability": "x", "operation": "y"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": {"allowed": ["."]},
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	chatter := &sequencedLLMChatter{
		responses: []*llm.Response{
			{Content: incompleteResponse, Model: "test-model"},
			{Content: completeResponse, Model: "test-model"},
		},
	}

	statusMock := &mockStatusUpdater{}
	traceMock := &mockTraceRecorder{}

	step := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", AutonomyLevel: "high"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
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
	if chatter.calls != 2 {
		t.Errorf("expected the loop to retry once after the incomplete spec (2 chat calls), got %d", chatter.calls)
	}
	if result["complexity"] != "easy" {
		t.Errorf("expected complexity easy, got: %v", result["complexity"])
	}

	var analysis models.TaskAnalysis
	if errUnmarshal := json.Unmarshal(task.Analysis, &analysis); errUnmarshal != nil {
		t.Fatalf("failed to unmarshal saved analysis: %v", errUnmarshal)
	}
	if analysis.PrimaryCategory != "backend" {
		t.Errorf("expected category backend from the eventually-accepted spec, got: %s", analysis.PrimaryCategory)
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

func TestAnalyzeStep_BoundaryCoverageValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-step-boundary-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:         "task-boundary",
		ProjectID:  "proj-boundary",
		Complexity: "easy",
		Status:     models.TaskStatusTodo,
	}

	// 1. Uncovered affected files or target_files
	uncoveredResponse := `
{
  "complexity": "easy",
  "primary_category": "backend",
  "scope": "Test boundary coverage",
  "affected_files": [{"file": "cmd/zentao-sync/main.go", "confidence": 1.0, "reason": "edit"}],
  "risks": [],
  "risk_domains": [],
  "execution_phases": [{"phase": "Phase 1: Setup", "tasks": ["write code"]}],
  "clarification_questions": [],
  "required_skills": [],
  "required_skills_map": {},
  "execution_units": [
    {
      "id": "u1",
      "objective": "write entrypoint",
      "tasks": ["Task 1.1: write code"],
      "execution_profile": {"agent": "backend", "skills": []},
      "constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"},
      "dependencies": [],
      "target_files": ["cmd/zentao-sync/main.go"]
    }
  ],
  "execution_irs": [{"node_id": "u1", "intent": {"capability": "x", "operation": "y"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": [{"module": "main", "root": "internal/"}],
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	chatterUncovered := &mockLLMChatter{
		resp: &llm.Response{
			Content: uncoveredResponse,
			Model:   "test-model",
		},
	}

	stepUncovered := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", AutonomyLevel: "high"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
		&mockProjectReader{project: &models.Project{DefaultAutonomy: "high"}},
		chatterUncovered,
		&mockPromptAssembler{},
		&mockSandboxRunner{},
		&mockArtifactSaver{},
		&mockStatusUpdater{},
		&mockTraceRecorder{},
		&mockLogger{},
		nil,
		nil,
		8.0,
		tools.DefaultRegistry(nil, nil),
	)

	_, err = stepUncovered.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error due to boundary coverage violation, got nil")
	}
	if !strings.Contains(err.Error(), "Boundary coverage validation failed") {
		t.Errorf("expected boundary coverage failure message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "cmd/zentao-sync/main.go") {
		t.Errorf("expected uncovered file name in error message, got: %v", err)
	}

	// 2. Empty target_files rejected
	emptyTargetResponse := `
{
  "complexity": "easy",
  "primary_category": "backend",
  "scope": "Test boundary coverage",
  "affected_files": [{"file": "internal/main.go", "confidence": 1.0, "reason": "edit"}],
  "risks": [],
  "risk_domains": [],
  "execution_phases": [{"phase": "Phase 1: Setup", "tasks": ["write code"]}],
  "clarification_questions": [],
  "required_skills": [],
  "required_skills_map": {},
  "execution_units": [
    {
      "id": "u1",
      "objective": "write entrypoint",
      "tasks": ["Task 1.1: write code"],
      "execution_profile": {"agent": "backend", "skills": []},
      "constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"},
      "dependencies": [],
      "target_files": []
    }
  ],
  "execution_irs": [{"node_id": "u1", "intent": {"capability": "x", "operation": "y"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": [{"module": "main", "root": "internal/"}],
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	chatterEmptyTarget := &mockLLMChatter{
		resp: &llm.Response{
			Content: emptyTargetResponse,
			Model:   "test-model",
		},
	}

	stepEmptyTarget := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", AutonomyLevel: "high"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
		&mockProjectReader{project: &models.Project{DefaultAutonomy: "high"}},
		chatterEmptyTarget,
		&mockPromptAssembler{},
		&mockSandboxRunner{},
		&mockArtifactSaver{},
		&mockStatusUpdater{},
		&mockTraceRecorder{},
		&mockLogger{},
		nil,
		nil,
		8.0,
		tools.DefaultRegistry(nil, nil),
	)

	_, err = stepEmptyTarget.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error for empty target_files: %v", err)
	}

	// 3. Covered output passes
	coveredResponse := `
{
  "complexity": "easy",
  "primary_category": "backend",
  "scope": "Test boundary coverage",
  "affected_files": [{"file": "internal/main.go", "confidence": 1.0, "reason": "edit"}],
  "risks": [],
  "risk_domains": [],
  "execution_phases": [{"phase": "Phase 1: Setup", "tasks": ["write code"]}],
  "clarification_questions": [],
  "required_skills": [],
  "required_skills_map": {},
  "execution_units": [
    {
      "id": "u1",
      "objective": "write entrypoint",
      "tasks": ["Task 1.1: write code"],
      "execution_profile": {"agent": "backend", "skills": []},
      "constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"},
      "dependencies": [],
      "target_files": ["internal/main.go"]
    }
  ],
  "execution_irs": [{"node_id": "u1", "intent": {"capability": "x", "operation": "y"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": [{"module": "main", "root": "internal/"}],
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	chatterCovered := &mockLLMChatter{
		resp: &llm.Response{
			Content: coveredResponse,
			Model:   "test-model",
		},
	}

	stepCovered := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", AutonomyLevel: "high"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
		&mockProjectReader{project: &models.Project{DefaultAutonomy: "high"}},
		chatterCovered,
		&mockPromptAssembler{},
		&mockSandboxRunner{},
		&mockArtifactSaver{},
		&mockStatusUpdater{},
		&mockTraceRecorder{},
		&mockLogger{},
		nil,
		nil,
		8.0,
		tools.DefaultRegistry(nil, nil),
	)

	_, err = stepCovered.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error for covered output: %v", err)
	}
}
