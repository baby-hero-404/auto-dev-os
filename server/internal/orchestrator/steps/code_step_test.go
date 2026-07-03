package steps

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockBackendAgentAssigner struct {
	agent       *models.Agent
	err         error
	releasedIDs []string
	calls       int
}

func (m *mockBackendAgentAssigner) AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	m.calls++
	return m.agent, m.err
}

func (m *mockBackendAgentAssigner) Release(ctx context.Context, agentID string) error {
	m.releasedIDs = append(m.releasedIDs, agentID)
	return nil
}

type mockFrontendAgentAssigner struct {
	agent       *models.Agent
	err         error
	releasedIDs []string
	calls       int
}

func (m *mockFrontendAgentAssigner) AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	m.calls++
	return m.agent, m.err
}

func (m *mockFrontendAgentAssigner) Release(ctx context.Context, agentID string) error {
	m.releasedIDs = append(m.releasedIDs, agentID)
	return nil
}

func TestCodeBackendStep_ExecutesAndSavesArtifacts(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1", Name: "Default Planner Agent", Role: models.AgentRolePlanner}
	assigner := &mockBackendAgentAssigner{agent: &models.Agent{ID: "assigned-be-1", Name: "Assigned BE", Role: models.AgentRoleBackend}}
	llmMock := &mockLLMRunner{
		result: StepResult{
			"parsed": map[string]any{
				"patch": "diff --git a/file.go b/file.go\n+new content\n",
			},
		},
	}
	artifactMock := &mockArtifactSaver{}
	worktreeMock := &mockWorktreeManager{
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
			// mock call
		},
	}

	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		llmMock,
		assigner,
		worktreeMock,
		&mockPatchApplier{},
		&mockDiffCapturer{},
		&mockStepWorkspaceLoader{},
		artifactMock,
		&mockTestRunner{},
		nil,
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assigner.releasedIDs) != 1 {
		t.Fatalf("expected backend agent release, got %d releases", len(assigner.releasedIDs))
	}
	if assigner.releasedIDs[0] != "assigned-be-1" {
		t.Fatalf("expected backend release of assigned-be-1, got %s", assigner.releasedIDs[0])
	}

	if !artifactMock.called {
		t.Error("expected artifact to be saved")
	}
	if artifactMock.artType != "patch" {
		t.Errorf("expected patch artifact, got: %s", artifactMock.artType)
	}
}

func TestCodeBackendStep_ReusesCurrentBackendAgent(t *testing.T) {
	task := &models.Task{
		ID:         "task-reuse-be",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1", Name: "Current Backend Agent", Role: models.AgentRoleBackend}
	assigner := &mockBackendAgentAssigner{agent: &models.Agent{ID: "assigned-be-1", Name: "Assigned BE", Role: models.AgentRoleBackend}}
	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{}}},
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{},
		&mockStepWorkspaceLoader{},
		&mockArtifactSaver{},
		&mockTestRunner{},
		nil,
		&mockLogger{},
	)

	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assigner.calls != 0 {
		t.Fatalf("expected current backend agent to be reused, assigner called %d times", assigner.calls)
	}
	if len(assigner.releasedIDs) != 0 {
		t.Fatalf("expected no borrowed backend release, got %d releases", len(assigner.releasedIDs))
	}
}

func TestCodeFrontendStep_SkipsOnEasyTask(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityEasy,
	}
	step := NewCodeFrontendStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleFrontend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		&mockLogger{},
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

func TestCodeFrontendStep_SkipsOnNoFrontendFiles(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []string{"backend/main.go", "db/schema.sql"},
	}
	analysisBytes, _ := json.Marshal(analysis)

	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisBytes,
	}
	step := NewCodeFrontendStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleFrontend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		&mockLogger{},
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

func TestCodeFrontendStep_ReleasesBorrowedAgent(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []string{"web/src/app/page.tsx"},
	}
	analysisBytes, _ := json.Marshal(analysis)

	task := &models.Task{
		ID:         "task-fe-release",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisBytes,
	}
	assigner := &mockFrontendAgentAssigner{agent: &models.Agent{ID: "assigned-fe-1", Name: "Assigned FE", Role: models.AgentRoleFrontend}}
	step := NewCodeFrontendStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRolePlanner}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{"patch": "diff --git a/file.tsx b/file.tsx\n+new content\n"}}},
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{},
		&mockStepWorkspaceLoader{},
		&mockArtifactSaver{},
		&mockTestRunner{},
		nil,
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assigner.releasedIDs) != 1 {
		t.Fatalf("expected frontend agent release, got %d releases", len(assigner.releasedIDs))
	}
	if assigner.releasedIDs[0] != "assigned-fe-1" {
		t.Fatalf("expected frontend release of assigned-fe-1, got %s", assigner.releasedIDs[0])
	}
}

func TestCodeFrontendStep_ReusesCurrentFrontendAgent(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []string{"web/src/app/page.tsx"},
	}
	analysisBytes, _ := json.Marshal(analysis)

	task := &models.Task{
		ID:         "task-reuse-fe",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisBytes,
	}
	assigner := &mockFrontendAgentAssigner{agent: &models.Agent{ID: "assigned-fe-1", Name: "Assigned FE", Role: models.AgentRoleFrontend}}
	step := NewCodeFrontendStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleFrontend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{}}},
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{},
		&mockStepWorkspaceLoader{},
		&mockArtifactSaver{},
		&mockTestRunner{},
		nil,
		&mockLogger{},
	)

	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assigner.calls != 0 {
		t.Fatalf("expected current frontend agent to be reused, assigner called %d times", assigner.calls)
	}
	if len(assigner.releasedIDs) != 0 {
		t.Fatalf("expected no borrowed frontend release, got %d releases", len(assigner.releasedIDs))
	}
}
