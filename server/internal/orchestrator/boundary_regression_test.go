package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestAnalyze_BoundaryViolation_ExhaustsBudget_FailsBeforeCoding(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch-reg-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:          "task-reg-1",
		ProjectID:   "proj-reg-1",
		Title:       "Test Regression Hard Failure",
		Description: "Reproduce e69924ba boundary violation failure",
		Complexity:  models.TaskComplexityEasy,
		SpecStatus:  models.TaskSpecStatusNone,
	}

	job := &models.WorkflowJob{
		ID:     "job-reg-1",
		TaskID: task.ID,
		Status: models.WorkflowJobStatusQueued,
	}

	agent := &models.Agent{
		ID:            "agent-reg-1",
		Name:          "Test Agent",
		Role:          models.AgentRoleBackend,
		AutonomyLevel: models.AgentAutonomyAutonomous,
	}

	repo := models.Repository{
		ID:        "repo-reg-1",
		ProjectID: "proj-reg-1",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
		Token:     "token-reg-1",
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{}
	artifactRepo := &mockArtifactRepo{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

	// steps.AnalyzeMaxIterations iterations budget. Every response returned contains the
	// uncovered boundary violation, so the loop must exhaust the *entire* current budget before
	// the caller sees the boundary-violation error as the final failure reason.
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
      "id": "code_backend_0",
      "objective": "write entrypoint",
      "tasks": ["Task 1.1: write code"],
      "execution_profile": {"agent": "backend", "skills": []},
      "constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"},
      "dependencies": [],
      "target_files": ["cmd/zentao-sync/main.go"]
    }
  ],
  "execution_irs": [{"node_id": "code_backend_0", "intent": {"capability": "modify", "operation": "write entrypoint"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": [{"module": "main", "root": "internal/"}],
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	var queue []*llm.Response
	for i := 0; i < steps.AnalyzeMaxIterations; i++ {
		queue = append(queue, &llm.Response{
			Model:   "mock-model",
			Content: uncoveredResponse,
		})
	}

	llmResponses := map[string]string{
		"code_backend": `{"patch": "diff --git a/main.go b/main.go\n+backend code", "summary": "backend done"}`,
		"review":       `{"findings": []}`,
		"fix":          `{"patch": "diff --git a/main.go b/main.go\n+fixed code", "summary": "fixed bug"}`,
	}
	llmProvider := &mockLLMProvider{
		responses:     llmResponses,
		responseQueue: queue,
	}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithLLMProvider(llmProvider),
		WithGitOpsClient(gitOps),
		WithArtifactRepository(artifactRepo),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(tmpDir),
		WithMaxPhaseCost(8.0),
	)

	// Run execution - should fail at analyze due to boundary coverage budget exhaustion
	orch.run(context.Background(), job.ID)

	// Assert: job leaves failed
	if job.Status != models.WorkflowJobStatusFailed {
		t.Errorf("expected job status to be failed, got %s", job.Status)
	}

	// Assert: the failure error names cmd/zentao-sync/main.go
	if !strings.Contains(job.LastError, "cmd/zentao-sync/main.go") {
		t.Errorf("expected LastError to name 'cmd/zentao-sync/main.go', got %q", job.LastError)
	}

	// Assert: mockLLMProvider.calls contains zero code_backend or fix entries
	for _, call := range llmProvider.calls {
		if strings.Contains(call, "code_backend") || strings.Contains(call, "fix") {
			t.Errorf("expected zero code_backend/fix LLM calls on hard failure path, but got call: %q", call)
		}
	}
}

func TestAnalyze_BoundaryViolation_SelfRepairsOnFeedback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch-reg-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:          "task-reg-2",
		ProjectID:   "proj-reg-2",
		Title:       "Test Regression Self Repair",
		Description: "Reproduce e69924ba boundary violation self repair",
		Complexity:  models.TaskComplexityEasy,
		SpecStatus:  models.TaskSpecStatusNone,
	}

	job := &models.WorkflowJob{
		ID:     "job-reg-2",
		TaskID: task.ID,
		Status: models.WorkflowJobStatusQueued,
	}

	agent := &models.Agent{
		ID:            "agent-reg-2",
		Name:          "Test Agent",
		Role:          models.AgentRoleBackend,
		AutonomyLevel: models.AgentAutonomyAutonomous,
	}

	repo := models.Repository{
		ID:        "repo-reg-2",
		ProjectID: "proj-reg-2",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
		Token:     "token-reg-2",
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{}
	artifactRepo := &mockArtifactRepo{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

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
      "id": "code_backend_0",
      "objective": "write entrypoint",
      "tasks": ["Task 1.1: write code"],
      "execution_profile": {"agent": "backend", "skills": []},
      "constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"},
      "dependencies": [],
      "target_files": ["cmd/zentao-sync/main.go"]
    }
  ],
  "execution_irs": [{"node_id": "code_backend_0", "intent": {"capability": "modify", "operation": "write entrypoint"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": [{"module": "main", "root": "internal/"}],
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	correctedResponse := `
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
      "id": "code_backend_0",
      "objective": "write entrypoint",
      "tasks": ["Task 1.1: write code"],
      "execution_profile": {"agent": "backend", "skills": []},
      "constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"},
      "dependencies": [],
      "target_files": ["cmd/zentao-sync/main.go"]
    }
  ],
  "execution_irs": [{"node_id": "code_backend_0", "intent": {"capability": "modify", "operation": "write entrypoint"}, "budget": {"discovery": 1, "implementation": 1, "validation": 1}}],
  "proposal_md": "## Proposal",
  "specs_md": "## ADDED Requirements",
  "design_md": "## Design",
  "tasks_md": "## Tasks",
  "execution_boundaries": [{"module": "main", "root": "cmd/"}],
  "acceptance_criteria": [{"id": "AC-1", "expected": "ok"}]
}`

	queue := []*llm.Response{
		{Model: "mock-model", Content: uncoveredResponse},
		{Model: "mock-model", Content: correctedResponse},
	}

	llmResponses := map[string]string{
		"code_backend": `{"patch": "diff --git a/main.go b/main.go\n+backend code", "summary": "backend done"}`,
		"review":       `{"findings": []}`,
		"fix":          `{"patch": "diff --git a/main.go b/main.go\n+fixed code", "summary": "fixed bug"}`,
	}
	llmProvider := &mockLLMProvider{
		responses:     llmResponses,
		responseQueue: queue,
	}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithLLMProvider(llmProvider),
		WithGitOpsClient(gitOps),
		WithArtifactRepository(artifactRepo),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(tmpDir),
		WithMaxPhaseCost(8.0),
	)

	// Run execution - should fail first turn of analyze, then self-repair on turn 2 and proceed to coding
	orch.run(context.Background(), job.ID)

	// Assert: job successfully completed the coding stages (or paused at merge/PR since mock git/PR will succeed)
	if job.Status == models.WorkflowJobStatusFailed {
		t.Errorf("job failed unexpectedly: %s", job.LastError)
	}

	// Call log must contain queued (x2 for queue) then code_backend
	hasQueued := 0
	hasBackend := false
	for _, call := range llmProvider.calls {
		if call == "queued" {
			hasQueued++
		}
		if call == "code_backend" {
			hasBackend = true
		}
	}
	if hasQueued < 2 {
		t.Errorf("expected at least 2 queued analyze calls, got %d", hasQueued)
	}
	if !hasBackend {
		t.Errorf("expected workflow to proceed to code_backend after self-repairing")
	}

	// Assert: the persisted analysis contains the corrected boundaries
	var savedAnalysis models.TaskAnalysis
	if err := json.Unmarshal(task.Analysis, &savedAnalysis); err != nil {
		t.Fatalf("failed to unmarshal persisted analysis: %v", err)
	}
	if len(savedAnalysis.ExecutionBoundaries) == 0 || savedAnalysis.ExecutionBoundaries[0].Root != "cmd/" {
		t.Errorf("expected corrected boundary root 'cmd/', got %+v", savedAnalysis.ExecutionBoundaries)
	}

	// Assert: the second analyze prompt contains the corrective coverage-error text
	if len(llmProvider.history) < 2 {
		t.Fatalf("expected at least 2 LLM provider calls, got %d", len(llmProvider.history))
	}
	secondCallMessages := llmProvider.history[1]
	foundErrorFeedback := false
	for _, msg := range secondCallMessages {
		if msg.Role == "user" && strings.Contains(msg.Content, "Boundary coverage validation failed") {
			foundErrorFeedback = true
			break
		}
	}
	if !foundErrorFeedback {
		t.Errorf("expected second analyze prompt to contain corrective coverage-error feedback")
	}
}
