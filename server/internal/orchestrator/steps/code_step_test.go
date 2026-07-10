package steps

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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

type mockAffectedFileReader struct {
	content string
	ok      bool
}

func (m *mockAffectedFileReader) ReadAffectedFileContent(ctx context.Context, task *models.Task, file string) (string, bool) {
	return m.content, m.ok
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
				"summary": "implemented the feature via tool calls",
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
		&mockDiffCapturer{diffVal: "diff --git a/file.go b/file.go\n+new content\n"},
		&mockStepWorkspaceLoader{},
		artifactMock,
		&mockTestRunner{},
		nil,
		&mockLogger{},
		&mockAffectedFileReader{content: "mock code", ok: true},
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

	// Agentic mode: edits are already applied via tool calls, so no "patch" artifact is
	// produced anymore — the workspace diff (captured from real git state) is saved instead.
	if !artifactMock.called {
		t.Error("expected artifact to be saved")
	}
	if artifactMock.artType != "diff" {
		t.Errorf("expected diff artifact, got: %s", artifactMock.artType)
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
		&mockAffectedFileReader{content: "mock code", ok: true},
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

func TestCodeBackendStep_ReleasesAgentOnAssignmentFailure(t *testing.T) {
	task := &models.Task{
		ID:         "task-fail-assign",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1", Name: "Default Planner Agent", Role: models.AgentRolePlanner}

	// Mock assigner returning an agent AND an error
	assigner := &mockBackendAgentAssigner{
		agent: &models.Agent{ID: "assigned-be-1", Name: "Assigned BE", Role: models.AgentRoleBackend},
		err:   errors.New("db error after assign"),
	}

	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockLLMRunner{},
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{},
		&mockStepWorkspaceLoader{},
		&mockArtifactSaver{},
		&mockTestRunner{},
		nil,
		&mockLogger{},
		&mockAffectedFileReader{content: "mock code", ok: true},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error from step.Execute, got nil")
	}

	if len(assigner.releasedIDs) != 1 {
		t.Fatalf("expected backend agent release on failure, got %d releases", len(assigner.releasedIDs))
	}
	if assigner.releasedIDs[0] != "assigned-be-1" {
		t.Fatalf("expected release of assigned-be-1, got %s", assigner.releasedIDs[0])
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
		&mockAffectedFileReader{content: "mock code", ok: true},
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
		AffectedFiles: []models.AffectedFile{{File: "backend/main.go"}, {File: "db/schema.sql"}},
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
		&mockAffectedFileReader{content: "mock code", ok: true},
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
		AffectedFiles: []models.AffectedFile{{File: "web/src/app/page.tsx"}},
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
		&mockAffectedFileReader{content: "mock code", ok: true},
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
		AffectedFiles: []models.AffectedFile{{File: "web/src/app/page.tsx"}},
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
		&mockAffectedFileReader{content: "mock code", ok: true},
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

func TestCodeBackendStep_IncludesDirectoryScan(t *testing.T) {
	// Create a temp directory to simulate the worktree
	tmpDir, err := os.MkdirTemp("", "code-step-scan-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:         "task-scan-123",
		Complexity: models.TaskComplexityMedium,
	}

	// Compute physical root matching the logic in code_backend.go
	workspaceRoot := filepath.Dir(tmpDir)
	repoName := "my-repo"
	role := "backend"

	// Create directories
	physicalRoot := paths.NewOSWorkspacePaths(workspaceRoot).RepoWorktreeDir(task.ID, repoName, role).String()
	if err := os.MkdirAll(physicalRoot, 0755); err != nil {
		t.Fatalf("failed to create physical root: %v", err)
	}

	// Create a mock file in physicalRoot
	mockFileName := "mock_backend_file_xxx.go"
	if err := os.WriteFile(filepath.Join(physicalRoot, mockFileName), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write mock file: %v", err)
	}

	assigner := &mockBackendAgentAssigner{agent: &models.Agent{ID: "assigned-be-1", Role: models.AgentRoleBackend}}
	llmMock := &mockLLMRunner{
		result: StepResult{
			"parsed": map[string]any{
				"patch": "diff --git a/mock_backend_file_xxx.go b/mock_backend_file_xxx.go\n+new content\n",
			},
		},
	}

	wsLoader := &mockStepWorkspaceLoader{
		loadFunc: func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
			return &models.TaskWorkspace{
				Root: tmpDir, // Setting root to our temp directory
				Repos: []models.RepoWorkspace{
					{Name: "my-repo"},
				},
			}, nil
		},
	}

	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRolePlanner}, JobID: "j1"},
		&mockTaskReader{task: task},
		llmMock,
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{},
		wsLoader,
		&mockArtifactSaver{},
		&mockTestRunner{},
		nil,
		&mockLogger{},
		&mockAffectedFileReader{content: "mock code", ok: true},
	)

	_, err = step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the prompt instruction contains mock_backend_file_xxx.go
	if !strings.Contains(llmMock.lastInstruction, mockFileName) {
		t.Errorf("expected instruction to contain file name %q, got: %s", mockFileName, llmMock.lastInstruction)
	}
	if !strings.Contains(llmMock.lastInstruction, "=== Repository Structure ===") {
		t.Error("expected instruction to contain repository structure section header")
	}
}

func TestCodeBackendStep_PriorFilesPropagation(t *testing.T) {
	tmpDir := t.TempDir()

	task := &models.Task{
		ID:        "task-test-2",
		ProjectID: "proj-1",
		Title:     "Prior files prop test",
	}

	assigner := &mockBackendAgentAssigner{agent: &models.Agent{ID: "assigned-be-1", Role: models.AgentRoleBackend}}
	llmMock := &mockLLMRunner{
		result: StepResult{
			"parsed": map[string]any{
				"patch": "diff --git a/some_file.go b/some_file.go\n+new content\n",
			},
		},
	}

	wsLoader := &mockStepWorkspaceLoader{
		loadFunc: func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
			return &models.TaskWorkspace{
				Root: tmpDir,
				Repos: []models.RepoWorkspace{
					{Name: "my-repo"},
				},
			}, nil
		},
	}

	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRolePlanner}, JobID: "j1"},
		&mockTaskReader{task: task},
		llmMock,
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{changed: []string{"some_file.go"}},
		wsLoader,
		&mockArtifactSaver{},
		&mockTestRunner{},
		nil,
		&mockLogger{},
		&mockAffectedFileReader{content: "mock code", ok: true},
	)

	// Build StepContext with a prior step's outputs containing files_changed
	stepCtx := workflow.StepContext{
		StepID: "code_backend_1",
		Inputs: map[string]map[string]any{
			"code_backend_0": {
				"files_changed": []string{"prior_file_1.go", "prior_file_2.go"},
			},
		},
	}

	out, err := step.Execute(context.Background(), stepCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify files_changed output of this step
	outFiles, ok := out["files_changed"].([]string)
	if !ok || len(outFiles) != 1 || outFiles[0] != "some_file.go" {
		t.Fatalf("expected out[files_changed] to be ['some_file.go'], got %#v", out["files_changed"])
	}

	// Verify instruction contains Files Created/Modified by Prior Steps
	if !strings.Contains(llmMock.lastInstruction, "### Files Created/Modified by Prior Steps ###") {
		t.Error("expected instruction to contain Files Created/Modified by Prior Steps section")
	}
	if !strings.Contains(llmMock.lastInstruction, "- prior_file_1.go") {
		t.Error("expected instruction to list prior_file_1.go")
	}
	if !strings.Contains(llmMock.lastInstruction, "- prior_file_2.go") {
		t.Error("expected instruction to list prior_file_2.go")
	}
}

type retryPatchApplier struct {
	calls int
}

func (m *retryPatchApplier) Validate(ctx context.Context, task *models.Task, patchData string, worktreeSuffix string) []error {
	return nil
}

func (m *retryPatchApplier) ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	m.calls++
	if m.calls == 1 {
		return errors.New("main.go:12: compilation failed")
	}
	return nil
}

// flakyTestRunner fails targeted tests on the first call and succeeds afterward, used to
// exercise the agentic retry gate (edits are already applied via tool calls; the only
// remaining verification is the post-hoc targeted-test run).
type flakyTestRunner struct {
	calls int
}

func (m *flakyTestRunner) RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (StepResult, error) {
	m.calls++
	if m.calls == 1 {
		return nil, errors.New("main_test.go:5: assertion failed")
	}
	return StepResult{"status": "passed"}, nil
}

func TestCodeBackendStep_RetryDoesNotDuplicateAffectedFilesContext(t *testing.T) {
	task := &models.Task{
		ID:         "task-retry-test",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1", Name: "Default Planner Agent", Role: models.AgentRolePlanner}
	assigner := &mockBackendAgentAssigner{agent: &models.Agent{ID: "assigned-be-1", Name: "Assigned BE", Role: models.AgentRoleBackend}}
	llmMock := &mockLLMRunner{
		result: StepResult{
			"parsed": map[string]any{
				"summary": "implemented the change via tool calls",
			},
		},
	}

	mockReader := &mockAffectedFileReader{
		content: "package main\n\nfunc main() {\n\t// test code\n}",
		ok:      true,
	}

	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		llmMock,
		assigner,
		&mockWorktreeManager{},
		&retryPatchApplier{},
		&mockDiffCapturer{changed: []string{"main.go"}},
		&mockStepWorkspaceLoader{},
		&mockArtifactSaver{},
		&flakyTestRunner{},
		nil,
		&mockLogger{},
		mockReader,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The retry error message from the failed targeted test must still reach the LLM...
	expectedErrorMsg := "the automated tests failed"
	if !strings.Contains(llmMock.lastInstruction, expectedErrorMsg) {
		t.Errorf("expected instruction to contain retry error message %q, got: %s", expectedErrorMsg, llmMock.lastInstruction)
	}

	// ...but the step itself must no longer duplicate the affected-files dump into the
	// instruction string: llmrunner.Runner already injects it unconditionally on every
	// attempt (Issue 4: double context injection).
	unexpectedHeader := "### Workspace Affected Files ###"
	if strings.Contains(llmMock.lastInstruction, unexpectedHeader) {
		t.Errorf("expected instruction to NOT contain duplicated header %q (now handled by llmrunner.Runner), got: %s", unexpectedHeader, llmMock.lastInstruction)
	}
}

type trackInstructionsLLMRunner struct {
	instructions []string
	isRetry      []bool
	isSR         []bool
	result       StepResult
}

func (m *trackInstructionsLLMRunner) RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (StepResult, error) {
	// Import prompts inside or use imported prompts
	m.instructions = append(m.instructions, instruction)
	m.isRetry = append(m.isRetry, prompts.IsRetry(ctx))
	m.isSR = append(m.isSR, prompts.UseSearchReplace(ctx))
	return m.result, nil
}

// alwaysFailingTestRunner fails every targeted test run with an incrementing error message,
// used to exercise the agentic retry loop's sliding-window feedback (only the latest test
// failure should be present in the instruction, not all prior ones).
type alwaysFailingTestRunner struct {
	attempts int
}

func (f *alwaysFailingTestRunner) RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (StepResult, error) {
	f.attempts++
	return nil, errors.New("test failure number " + string(rune('0'+f.attempts)))
}

func TestCodeBackendStep_SlidingWindowRetry(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1", Name: "Default Planner Agent", Role: models.AgentRolePlanner}
	assigner := &mockBackendAgentAssigner{agent: &models.Agent{ID: "assigned-be-1", Name: "Assigned BE", Role: models.AgentRoleBackend}}

	llmMock := &trackInstructionsLLMRunner{
		result: StepResult{
			"parsed": map[string]any{
				"summary": "implemented the change via tool calls",
			},
		},
	}

	tester := &alwaysFailingTestRunner{}

	step := NewCodeBackendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		llmMock,
		assigner,
		&mockWorktreeManager{},
		&mockPatchApplier{},
		&mockDiffCapturer{changed: []string{"main.go"}},
		&mockStepWorkspaceLoader{},
		&mockArtifactSaver{},
		tester,
		nil,
		&mockLogger{},
		&mockAffectedFileReader{content: "package main\n\nfunc main() {}", ok: true},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error from step due to repeated targeted test failures")
	}

	// Should have run 3 attempts
	if len(llmMock.instructions) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(llmMock.instructions))
	}

	// 1st attempt: should be the base instruction.
	// 2nd attempt: should contain "test failure number 1".
	// 3rd attempt: should contain "test failure number 2".
	// CRITICAL: 3rd attempt should NOT contain "test failure number 1"!
	inst1 := llmMock.instructions[0]
	inst2 := llmMock.instructions[1]
	inst3 := llmMock.instructions[2]

	if strings.Contains(inst1, "test failure number") {
		t.Error("1st attempt should not contain a test failure message")
	}

	if !strings.Contains(inst2, "test failure number 1") {
		t.Error("2nd attempt should contain failure 1")
	}

	if !strings.Contains(inst3, "test failure number 2") {
		t.Error("3rd attempt should contain failure 2")
	}

	if strings.Contains(inst3, "test failure number 1") {
		t.Error("3rd attempt should NOT contain failure 1 (sliding window violation)")
	}

	// Context propagation checks:
	if llmMock.isRetry[0] || llmMock.isSR[0] {
		t.Error("1st attempt context: isRetry and isSR should be false")
	}
	if !llmMock.isRetry[1] || llmMock.isSR[1] {
		t.Error("2nd attempt context: isRetry should be true, isSR should be false")
	}
	// Agentic mode never switches to the SEARCH/REPLACE text convention — edits happen via
	// native tool calls, so isSR should stay false even on the final attempt.
	if !llmMock.isRetry[2] || llmMock.isSR[2] {
		t.Error("3rd attempt context: isRetry should be true, isSR should remain false in agentic mode")
	}

	// Agentic mode must never fall back to the SEARCH/REPLACE text instructions.
	if strings.Contains(inst2, "SEARCH/REPLACE block format") || strings.Contains(inst3, "SEARCH/REPLACE block format") {
		t.Error("agentic mode should never switch to the SEARCH/REPLACE format instructions")
	}
}
