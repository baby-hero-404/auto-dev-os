package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestOrchestrator_Run_Integration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:          "task-123",
		ProjectID:   "proj-123",
		Title:       "Test Task",
		Description: "Write tests for the server package.",
		Complexity:  models.TaskComplexityMedium,
		SpecStatus:  models.TaskSpecStatusApproved,
	}

	job := &models.WorkflowJob{
		ID:     "job-123",
		TaskID: "task-123",
		Status: models.WorkflowJobStatusQueued,
	}

	agent := &models.Agent{
		ID:   "agent-123",
		Name: "Test Agent",
		Role: models.AgentRoleBackend,
	}

	repo := models.Repository{
		ID:        "repo-123",
		ProjectID: "proj-123",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
		Token:     "token-123",
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{}
	artifactRepo := &mockArtifactRepo{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

	llmResponses := map[string]string{
		"plan":          `{"plan": "step 1 plan"}`,
		"code_backend":  `{"patch": "diff --git a/main.go b/main.go\n+backend code", "summary": "backend done"}`,
		"code_frontend": `{"patch": "diff --git a/ui.js b/ui.js\n+frontend code", "summary": "frontend done"}`,
		"review":        `{"findings": []}`,
		"fix":           `{"patch": "diff --git a/main.go b/main.go\n+fixed code", "summary": "fixed bug"}`,
	}
	llmProvider := &mockLLMProvider{responses: llmResponses}

	// Initialize Orchestrator
	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithLLMProvider(llmProvider),
		WithGitOpsClient(gitOps),
		WithArtifactRepository(artifactRepo),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(tmpDir),
	)

	// Run execution
	orch.run(context.Background(), "job-123")

	// 1. Verify repository was cloned
	if gitOps.clonedRepo != "https://github.com/test/repo.git" {
		t.Errorf("expected repo clone, got %s", gitOps.clonedRepo)
	}

	// 2. Verify git diff command executions in sandbox. Coding steps are agentic now (Issue
	// 1+2): edits are applied via native tool calls rather than a sandboxed "git apply
	// patch.diff" command, so only the post-hoc diff capture still goes through the sandbox.
	capturedDiff := false
	for _, cmd := range sandboxRuntime.commands {
		if cmd == "git diff" || strings.Contains(cmd, "git diff") {
			capturedDiff = true
		}
	}
	if !capturedDiff {
		t.Error("expected 'git diff' to have been executed in sandbox")
	}

	// 3. Verify GitOps PR was created
	if gitOps.createdBranch != "feature/task-123" {
		t.Errorf("expected branch feature/task-123, got %s", gitOps.createdBranch)
	}
	if gitOps.prTitle != "AutoCodeOS: Test Task" {
		t.Errorf("expected PR title AutoCodeOS: Test Task, got %s", gitOps.prTitle)
	}

	// 4. Verify artifacts were saved in DB. Agentic coding steps no longer produce a "patch"
	// artifact (edits are applied directly via tool calls); the workspace diff is still
	// captured and saved.
	expectedTypes := map[string]bool{
		"prompt":          false,
		"llm_response":    false,
		"diff":            false,
		"review_findings": false,
		"test_output":     false,
	}

	for _, art := range artifactRepo.artifacts {
		if art.JobID != "job-123" {
			t.Errorf("expected JobID job-123, got %s", art.JobID)
		}
		if art.TaskID != "task-123" {
			t.Errorf("expected TaskID task-123, got %s", art.TaskID)
		}
		expectedTypes[art.Type] = true
	}

	for k, found := range expectedTypes {
		if !found {
			t.Errorf("expected artifact type %s to be saved, but was not found", k)
		}
	}
}

// TestOrchestrator_Run_PropagatesStateMachineEnabledFlag guards against the flag-gated
// migration (docs/openspecs/execution-semantics-2026/tasks.md Task 2.2) silently becoming a
// no-op: WithStateMachineEnabled(true) must actually reach models.IsStateMachineEnabled(ctx)
// at LLM call time, not just get stored on the Orchestrator struct. Previously
// o.stateMachineEnabled was read nowhere — the flag never made it into ctx via
// models.StateMachineEnabledCtxKey, so enabling it in config had zero runtime effect.
func TestOrchestrator_Run_PropagatesStateMachineEnabledFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:          "task-123",
		ProjectID:   "proj-123",
		Title:       "Test Task",
		Description: "Write tests for the server package.",
		Complexity:  models.TaskComplexityMedium,
		SpecStatus:  models.TaskSpecStatusApproved,
	}

	job := &models.WorkflowJob{
		ID:     "job-123",
		TaskID: "task-123",
		Status: models.WorkflowJobStatusQueued,
	}

	agent := &models.Agent{
		ID:   "agent-123",
		Name: "Test Agent",
		Role: models.AgentRoleBackend,
	}

	repo := models.Repository{
		ID:        "repo-123",
		ProjectID: "proj-123",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
		Token:     "token-123",
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{}
	artifactRepo := &mockArtifactRepo{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

	llmResponses := map[string]string{
		"plan":          `{"plan": "step 1 plan"}`,
		"code_backend":  `{"patch": "diff --git a/main.go b/main.go\n+backend code", "summary": "backend done"}`,
		"code_frontend": `{"patch": "diff --git a/ui.js b/ui.js\n+frontend code", "summary": "frontend done"}`,
		"review":        `{"findings": []}`,
		"fix":           `{"patch": "diff --git a/main.go b/main.go\n+fixed code", "summary": "fixed bug"}`,
	}
	llmProvider := &mockLLMProvider{responses: llmResponses}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithLLMProvider(llmProvider),
		WithGitOpsClient(gitOps),
		WithArtifactRepository(artifactRepo),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(tmpDir),
		WithStateMachineEnabled(true),
	)

	orch.run(context.Background(), "job-123")

	if !llmProvider.sawStateMachineEnabled {
		t.Error("expected models.IsStateMachineEnabled(ctx) to be true during step execution when WithStateMachineEnabled(true) is set — the flag never reached ctx")
	}
}

func TestOrchestrator_Run_ReleasesAgentWhenFailureOccursAfterAssign(t *testing.T) {
	task := &models.Task{
		ID:          "task-release",
		ProjectID:   "proj-release",
		Title:       "Release Agent",
		Description: "Fail after assigning the agent.",
		Status:      models.TaskStatusContextLoading,
		Complexity:  models.TaskComplexityMedium,
		SpecStatus:  models.TaskSpecStatusApproved,
	}
	job := &models.WorkflowJob{
		ID:     "job-release",
		TaskID: task.ID,
		Status: models.WorkflowJobStatusQueued,
	}
	agent := &models.Agent{
		ID:   "agent-release",
		Name: "Release Agent",
		Role: models.AgentRoleBackend,
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job, agentUpdateErr: errors.New("persist assigned agent failed")}
	agentAssigner := &mockAgentAssigner{agent: agent}
	orch := New(taskRepo, workflowRepo, agentAssigner, &mockSandboxRuntime{})

	orch.run(context.Background(), job.ID)

	if len(agentAssigner.releasedIDs) != 1 {
		t.Fatalf("expected assigned agent to be released once, got %d releases", len(agentAssigner.releasedIDs))
	}
	if agentAssigner.releasedIDs[0] != agent.ID {
		t.Fatalf("expected release of agent %s, got %s", agent.ID, agentAssigner.releasedIDs[0])
	}
	if job.Status != models.WorkflowJobStatusFailed {
		t.Fatalf("expected job status failed, got %s", job.Status)
	}
	if task.Status != models.TaskStatusFailed {
		t.Fatalf("expected task status failed, got %s", task.Status)
	}
}

func TestOrchestrator_StepPR_NoChangesMerged(t *testing.T) {
	task := &models.Task{
		ID:          "task-no-change",
		ProjectID:   "proj-no-change",
		Title:       "No change task",
		Description: "Nothing should be committed.",
		Complexity:  models.TaskComplexityMedium,
		Status:      models.TaskStatusTesting,
	}
	job := &models.WorkflowJob{ID: "job-no-change", TaskID: task.ID, Status: models.WorkflowJobStatusQueued}
	agent := &models.Agent{ID: "agent-123", Name: "Test Agent", Role: models.AgentRoleBackend}
	repo := models.Repository{ID: "repo-123", ProjectID: task.ProjectID, URL: "https://github.com/test/repo.git", Branch: "main", Token: "token-123"}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{prURLSet: true, prURL: ""}
	artifactRepo := &mockArtifactRepo{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithGitOpsClient(gitOps),
		WithArtifactRepository(artifactRepo),
		WithRepositoryRepository(reposRepo),
	)

	runners := orch.stepRunners(task, agent, job.ID, job.Step)
	runner := runners[workflow.StepPR]
	if runner == nil {
		t.Fatal("missing PR step runner")
	}

	_, err := runner(context.Background(), workflow.StepContext{Inputs: map[string]map[string]any{
		workflow.StepTest: map[string]any{"status": "passed"},
	}})
	if !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Fatalf("expected workflow.ErrWaitingApproval, got: %v", err)
	}
	if task.Status != models.TaskStatusPrReady {
		t.Fatalf("expected task status pr_ready, got %s", task.Status)
	}
	if len(taskRepo.updated) == 0 {
		t.Fatal("expected task updates to be recorded")
	}
	for _, update := range taskRepo.updated {
		if update.PRURLs != nil {
			t.Fatalf("expected no PR URL update when no changes are detected, got %v", []string(*update.PRURLs))
		}
		if update.Status != nil && *update.Status != models.TaskStatusPrReady {
			t.Fatalf("expected only pr_ready status update, got %s", *update.Status)
		}
	}
}

func TestParseJSONMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw json",
			input:    `{"a": 1}`,
			expected: `{"a": 1}`,
		},
		{
			name:     "markdown json",
			input:    "```json\n{\"a\": 2}\n```",
			expected: `{"a": 2}`,
		},
		{
			name:     "markdown without language prefix",
			input:    "```\n{\"a\": 3}\n```",
			expected: `{"a": 3}`,
		},
		{
			name:     "json embedded in text",
			input:    "Sure, here is the result:\n{\n  \"a\": 4\n}\nHope this helps!",
			expected: `{"a": 4}`,
		},
		{
			name:     "json array",
			input:    `[{"a": 1}]`,
			expected: `{"array": [{"a": 1}]}`,
		},
		{
			name:     "json array in markdown",
			input:    "```json\n[{\"a\": 2}]\n```",
			expected: `{"array": [{"a": 2}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := llmrunner.ParseJSONMarkdown(tc.input)
			if err != nil {
				t.Fatalf("llmrunner.ParseJSONMarkdown failed: %v", err)
			}
			raw, _ := json.Marshal(res)
			var expectedMap map[string]any
			_ = json.Unmarshal([]byte(tc.expected), &expectedMap)
			expectedRaw, _ := json.Marshal(expectedMap)
			if string(raw) != string(expectedRaw) {
				t.Errorf("got %s, want %s", string(raw), string(expectedRaw))
			}
		})
	}
}

func TestOrchestrator_RemoveWorkspaceSafety(t *testing.T) {
	root := t.TempDir()
	orch := New(nil, nil, nil, nil, WithWorkspaceRoot(root))
	orch.initWkspace()

	if err := os.MkdirAll(filepath.Join(root, "task-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := orch.wkspace.RemoveWorkspace("task-1"); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "task-1")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace removed, got err=%v", err)
	}
	if err := orch.wkspace.RemoveWorkspace(""); err == nil {
		t.Fatal("expected empty task id to be rejected")
	}
	if err := orch.wkspace.RemoveWorkspace("../outside"); err == nil {
		t.Fatal("expected escaping task id to be rejected")
	}
}

func TestOrchestrator_PruneWorkspacesHonorsRetention(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "old-task")
	newDir := filepath.Join(root, "new-task")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	orch := New(nil, nil, nil, nil,
		WithWorkspaceRoot(root),
		WithWorkspaceRetention(time.Hour, time.Hour),
	)
	orch.initWkspace()

	removed, err := orch.wkspace.PruneWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("prune workspaces: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed workspace, got %d", removed)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("expected old workspace removed, got err=%v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("expected new workspace retained: %v", err)
	}
}

func TestOrchestrator_PruneLogsHonorsRetention(t *testing.T) {
	root := t.TempDir()
	oldFile := filepath.Join(root, "old-task.jsonl")
	newFile := filepath.Join(root, "new-task.jsonl")
	if err := os.WriteFile(oldFile, []byte("log line 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("log line 2"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -15) // older than 14 days
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	removed, err := wkspace.PruneLogFiles(context.Background(), 14, root)
	if err != nil {
		t.Fatalf("prune log files: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed log file, got %d", removed)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old log file removed, got err=%v", err)
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("expected new log file retained: %v", err)
	}
}

func TestMatchAffectedFile(t *testing.T) {
	tests := []struct {
		pattern  string
		file     string
		expected bool
	}{
		// Exact Match
		{"internal/app/backend/handlers.go", "internal/app/backend/handlers.go", true},
		{"readme.md", "code/repos/test/main/readme.md", true},
		{"Dockerfile", "code/repos/test/main/Dockerfile", true},
		{"src/readme.md", "code/repos/test/main/src/readme.md", true},
		{"internal/app/backend/handlers.go", "internal/app/backend/other.go", false},

		// Directory Prefix Match
		{"internal/app/backend", "internal/app/backend/handlers.go", true},
		{"internal/app/backend", "internal/app/backend/models/user.go", true},
		{"internal/app/backend", "internal/app/backend-other/handlers.go", false},

		// Glob Match
		{"*.go", "main.go", true},
		{"*.go", "internal/app/backend/handlers.go", false},             // base name match fallback disabled for safety
		{"internal/**/*.go", "internal/app/backend/handlers.go", false}, // filepath.Match doesn't support ** recursive glob

		// Extension / Description Matches (Should now be false/rejected)
		{"New GoLang source files (.go)", "internal/app/backend/handlers.go", false},
		{"New GoLang source files (.go)", "go.mod", false},
		{"Documentation files (if applicable)", "README.md", false},

		// Catch-all (Should now be false/rejected)
		{"All relevant source files of the original project (language unknown)", "internal/app/backend/handlers.go", false},
		{"All relevant source files", "package.json", false},

		// Filename Inclusion (Should now be false/rejected)
		{"New GoLang project configuration files (e.g., go.mod, Dockerfile for Go)", "go.mod", false},
		{"New GoLang project configuration files (e.g., go.mod, Dockerfile for Go)", "Dockerfile", false},
	}

	for i, tc := range tests {
		actual := patch.MatchAffectedFile(tc.pattern, tc.file)
		if actual != tc.expected {
			t.Errorf("Test %d failed: patch.MatchAffectedFile(%q, %q) expected %v, got %v", i, tc.pattern, tc.file, tc.expected, actual)
		}
	}
}

func TestOrchestrator_StepFix_LoopAndLimit(t *testing.T) {
	task := &models.Task{
		ID:         "task-loop",
		ProjectID:  "proj-loop",
		Title:      "Test Loop Task",
		Complexity: models.TaskComplexityMedium,
		Status:     models.TaskStatusCoding,
	}

	agent := &models.Agent{
		ID:   "agent-loop",
		Role: models.AgentRoleBackend,
	}

	taskRepo := &mockTaskRepo{task: task}
	job := &models.WorkflowJob{ID: "job-loop"}
	workflowRepo := &mockWorkflowRepo{job: job}

	llmResponses := map[string]string{
		"fix": `{"patch": "diff --git a/main.go b/main.go\n+fix", "fixes_applied": true, "summary": "fixed the finding"}`,
	}
	mockLLM := &mockLLMProvider{responses: llmResponses}
	sandboxRuntime := &mockSandboxRuntime{}

	orch := New(taskRepo, workflowRepo, nil, sandboxRuntime, WithLLMProvider(mockLLM))

	runners := orch.stepRunners(task, agent, "job-loop", "")
	fixRunner := runners[workflow.StepFix]

	// 1. Run without cycle limit reached
	ctx := context.Background()
	stepCtx := workflow.StepContext{
		Inputs: map[string]map[string]any{
			workflow.StepReview: {
				"cycle_limit_reached": false,
				"parsed": map[string]any{
					"findings": []any{map[string]any{"file": "main.go", "severity": "high"}},
				},
			},
		},
	}
	res, err := fixRunner(ctx, stepCtx)
	if !errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Errorf("expected ErrReviewFixLoop, got err=%v, res=%v", err, res)
	}

	// 2. Run with cycle limit reached
	stepCtxLimit := workflow.StepContext{
		Inputs: map[string]map[string]any{
			workflow.StepReview: {
				"cycle_limit_reached": true,
				"parsed": map[string]any{
					"findings": []any{map[string]any{"file": "main.go", "severity": "high"}},
				},
			},
		},
	}
	resLimit, errLimit := fixRunner(ctx, stepCtxLimit)
	if errLimit != nil {
		t.Errorf("expected no error when limit is reached, got err=%v", errLimit)
	}
	if resLimit["status"] != "skipped" {
		t.Errorf("expected status skipped, got %v", resLimit["status"])
	}
}

func TestOrchestrator_StepReview_CycleCountIgnoresNonSuccessCheckpoints(t *testing.T) {
	task := &models.Task{
		ID:         "task-review-cycle",
		ProjectID:  "proj-review-cycle",
		Title:      "Review cycle task",
		Complexity: models.TaskComplexityMedium,
		Status:     models.TaskStatusReviewing,
	}
	agent := &models.Agent{ID: "agent-review", Role: models.AgentRoleReviewer}

	workflowRepo := &mockWorkflowRepo{
		job: &models.WorkflowJob{ID: "job-review-cycle", TaskID: task.ID},
		checkpoints: []models.WorkflowCheckpoint{
			{TaskID: task.ID, Step: workflow.StepReview, State: []byte(`{"status":"running"}`)},
			{TaskID: task.ID, Step: workflow.StepReview, State: []byte(`{"status":"failed"}`)},
			{TaskID: task.ID, Step: workflow.StepReview, State: []byte(`{"status":"success"}`)},
			{TaskID: task.ID, Step: workflow.StepFix, State: []byte(`{"status":"success"}`)},
			{TaskID: task.ID, Step: workflow.StepReview, State: []byte(`{"status":"success"}`)},
		},
	}
	orch := New(&mockTaskRepo{task: task}, workflowRepo, &mockAgentAssigner{agent: agent}, &mockSandboxRuntime{},
		WithLLMProvider(&mockLLMProvider{responses: map[string]string{
			"Review": `{"findings":[{"severity":"high","file":"main.go","line":1,"recommendation":"fix it"}]}`,
		}}),
	)

	orch.initRepoutil()
	orch.initCheckpoints()
	orch.initWkspace()

	step := steps.NewReviewStep(
		steps.StepRuntime{Task: task, Agent: agent, JobID: "job-review-cycle"},
		orch.tasks,
		orch.projects,
		llmRunnerAdapter{run: orch.runLLMStep},
		orch.repoutil,
		artifactSaverAdapter{save: orch.checkpoints.SaveArtifact},
		orch.agents,
		orch.checkpoints,
		orch.workflows,
		statusUpdaterAdapter{update: orch.updateTaskStatus},
		loggerAdapter{log: orch.log},
	)

	out, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("executeStepReview returned error: %v", err)
	}
	if out["cycle_limit_reached"] == true {
		t.Fatal("cycle limit should not be reached when only two review checkpoints succeeded")
	}
	if task.Status != models.TaskStatusFixing {
		t.Fatalf("expected review with findings to move to fixing, got %s", task.Status)
	}
}

type testMockRepositoriesRepo struct {
	repos []models.Repository
}

func (m *testMockRepositoriesRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return m.repos, nil
}

func (m *testMockRepositoriesRepo) ListAll(ctx context.Context) ([]models.Repository, error) {
	return m.repos, nil
}

func TestOrchestrator_GetTaskRepoHostPath_FailsOnRepositoryIDMismatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-path-mismatch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	task := &models.Task{ID: "task-path", ProjectID: "proj-path", RepositoryID: &repoID}
	orch := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories: &testMockRepositoriesRepo{repos: []models.Repository{{
			ID:  "repo-b",
			URL: "https://github.com/example/repo-b.git",
		}}},
	}
	orch.initWkspace()
	orch.initRepoutil()

	ws := orch.wkspace.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: "repo-b",
		Name:   "repo-b",
		Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-b", "main")},
	}}
	if err := os.MkdirAll(ws.Root, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	if err := orch.wkspace.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	if _, err := orch.repoutil.GetTaskRepoHostPath(context.Background(), task); err == nil {
		t.Fatal("expected repository ID mismatch to fail fast")
	}
}

func TestOrchestrator_ReadAffectedFileContent_ResolvesRepoRelativePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "affected-file-root-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	task := &models.Task{ID: "task-affected", ProjectID: "proj-affected", RepositoryID: &repoID}
	orch := &Orchestrator{workspaceRoot: tmpDir}
	orch.initWkspace()
	ws := orch.wkspace.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: repoID,
		Name:   "repo-a",
		Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-a", "main")},
	}}
	repoRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
	if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := orch.wkspace.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	content, ok := orch.readAffectedFileContent(context.Background(), task, "src/main.go")
	if !ok {
		t.Fatal("expected repo-relative affected file to resolve")
	}
	if !strings.Contains(content, "package main") {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestOrchestrator_ReadAffectedFileContent_WorktreeFirst(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "affected-file-worktree-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	agentID := "agent-123"
	task := &models.Task{
		ID:           "task-affected-wt",
		ProjectID:    "proj-affected-wt",
		RepositoryID: &repoID,
		AgentID:      &agentID,
	}

	agent := &models.Agent{
		ID:   agentID,
		Role: "backend",
	}

	taskRepo := &mockTaskRepo{task: task}
	agentAssigner := &mockAgentAssigner{agent: agent}

	orch := New(taskRepo, nil, agentAssigner, nil, WithWorkspaceRoot(tmpDir))
	orch.initWkspace()
	ws := orch.wkspace.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: repoID,
		Name:   "repo-a",
		Paths: models.RepoWorkspacePaths{
			Main: filepath.Join("repos", "repo-a", "main"),
			Worktrees: map[string]string{
				"backend": filepath.Join("repos", "repo-a", "worktrees", "backend"),
			},
		},
	}}

	mainRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
	wtRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Worktrees["backend"])

	if err := os.MkdirAll(filepath.Join(mainRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create main repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(wtRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create wt repo dir: %v", err)
	}

	// Write different content to main vs worktree
	if err := os.WriteFile(filepath.Join(mainRoot, "src", "main.go"), []byte("package main\n// main\n"), 0o644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtRoot, "src", "main.go"), []byte("package main\n// worktree\n"), 0o644); err != nil {
		t.Fatalf("failed to write wt file: %v", err)
	}
	// Write a file that only exists in worktree
	if err := os.WriteFile(filepath.Join(wtRoot, "src", "wt_only.go"), []byte("package main\n// wt only\n"), 0o644); err != nil {
		t.Fatalf("failed to write wt only file: %v", err)
	}

	if err := orch.wkspace.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// 1. Verify we read the worktree version for a file in both
	content, ok := orch.readAffectedFileContent(context.Background(), task, "src/main.go")
	if !ok {
		t.Fatal("expected file to resolve")
	}
	if !strings.Contains(content, "// worktree") {
		t.Fatalf("expected worktree content, got %q", content)
	}

	// 2. Verify we read the file that only exists in worktree
	contentOnly, okOnly := orch.readAffectedFileContent(context.Background(), task, "src/wt_only.go")
	if !okOnly {
		t.Fatal("expected wt_only file to resolve")
	}
	if !strings.Contains(contentOnly, "// wt only") {
		t.Fatalf("expected wt only content, got %q", contentOnly)
	}
}

func TestOrchestrator_ClearCheckpointsForRepair(t *testing.T) {
	tests := []struct {
		name     string
		task     *models.Task
		expected []string
	}{
		{
			name: "PrunesDynamicSubsteps_MediumComplexity",
			task: &models.Task{
				ID:         "task-repair-123",
				ProjectID:  "proj-repair-123",
				Complexity: models.TaskComplexityMedium,
			},
			expected: []string{"code_backend", "code_frontend", "review", "fix", "test", "pr"},
		},
		{
			name: "EasyClearsCodingSteps",
			task: &models.Task{
				ID:         "task-repair-456",
				ProjectID:  "proj-repair-456",
				Complexity: models.TaskComplexityEasy,
			},
			expected: []string{"code_backend", "code_frontend", "review", "fix", "test", "pr"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taskRepo := &mockTaskRepo{task: tc.task}
			workflowRepo := &mockWorkflowRepo{}
			orch := New(taskRepo, workflowRepo, nil, nil)

			if err := orch.ClearCheckpointsForRepair(context.Background(), tc.task.ID); err != nil {
				t.Fatalf("ClearCheckpointsForRepair failed: %v", err)
			}

			if len(workflowRepo.deletedSteps) != len(tc.expected) {
				t.Fatalf("unexpected deleted step count: got %v want %v", workflowRepo.deletedSteps, tc.expected)
			}
			for i, step := range tc.expected {
				if workflowRepo.deletedSteps[i] != step {
					t.Fatalf("unexpected deleted step at %d: got %q want %q", i, workflowRepo.deletedSteps[i], step)
				}
			}
		})
	}
}

func TestOrchestrator_WorktreeAndPatchHelpers(t *testing.T) {
	// 1. Test splitPatchByRepo
	patchText := `Some header info
diff --git a/repo1/src/main.go b/repo1/src/main.go
index 123456..789012 100644
--- a/repo1/src/main.go
+++ b/repo1/src/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
diff --git a/repo2/index.js b/repo2/index.js
index abc..def 100644
--- a/repo2/index.js
+++ b/repo2/index.js
@@ -1,1 +1,2 @@
 console.log("hello");
+console.log("world");
`

	splitPatches := patch.SplitPatchByRepo(patchText)
	if len(splitPatches) != 2 {
		t.Errorf("expected 2 split patches, got: %d", len(splitPatches))
	}
	if !strings.Contains(splitPatches["repo1"], "src/main.go") {
		t.Errorf("expected repo1 patch to contain main.go, got: %s", splitPatches["repo1"])
	}
	if strings.Contains(splitPatches["repo1"], "index.js") {
		t.Errorf("repo1 patch should not contain repo2 changes")
	}
	if !strings.Contains(splitPatches["repo2"], "index.js") {
		t.Errorf("expected repo2 patch to contain index.js, got: %s", splitPatches["repo2"])
	}

	// 2. Test hostWorktreePath and containerPathForHostPath
	orch := &Orchestrator{
		workspaceRoot: "/tmp/workspaces",
	}
	orch.initRepoutil()

	// Single repo case
	repoID := "single-repo-id"
	singleTask := &models.Task{
		ID:           "task-1",
		RepositoryID: &repoID,
	}

	localPath := "/tmp/workspaces/task-1"
	repo := models.Repository{
		ID:  repoID,
		URL: "https://github.com/example/repo-a.git",
	}

	repoPathWithoutMetadata := orch.repoutil.RepoHostPath(singleTask, nil, repo)
	expectedRepoPathWithoutMetadata := filepath.Clean("/tmp/workspaces/task-1/code/repos/repo-a/main")
	if filepath.Clean(repoPathWithoutMetadata) != expectedRepoPathWithoutMetadata {
		t.Errorf("expected repo path without metadata: %s, got: %s", expectedRepoPathWithoutMetadata, repoPathWithoutMetadata)
	}

	// Suffix case
	bePath := orch.repoutil.HostWorktreePath(singleTask, localPath, "-be-worktree")
	expectedBePath := filepath.Clean("/tmp/workspaces/task-1/be")
	if filepath.Clean(bePath) != expectedBePath {
		t.Errorf("expected single-repo bePath: %s, got: %s", expectedBePath, bePath)
	}

	containerBe := orch.containerPathForHostPath(singleTask, bePath, "-be-worktree")
	if containerBe != "/workspace/be" {
		t.Errorf("expected containerPath for worktree to be /workspace/be, got: %s", containerBe)
	}

	// Child in worktree case
	containerBeFile := orch.containerPathForHostPath(singleTask, filepath.Join(bePath, "src/main.go"), "-be-worktree")
	if containerBeFile != "/workspace/be/src/main.go" {
		t.Errorf("expected container path /workspace/be/src/main.go, got: %s", containerBeFile)
	}

	// Multi repo case
	multiTask := &models.Task{
		ID:           "task-2",
		RepositoryID: nil,
	}

	multiRepoPathWithoutMetadata := orch.repoutil.RepoHostPath(multiTask, nil, repo)
	expectedMultiRepoPathWithoutMetadata := filepath.Clean("/tmp/workspaces/task-2/code/repos/repo-a/main")
	if filepath.Clean(multiRepoPathWithoutMetadata) != expectedMultiRepoPathWithoutMetadata {
		t.Errorf("expected multi repo path without metadata: %s, got: %s", expectedMultiRepoPathWithoutMetadata, multiRepoPathWithoutMetadata)
	}

	multiLocalPath := "/tmp/workspaces/task-2"
	multiRepoPath := filepath.Join(multiLocalPath, "repo-a")

	multiBePath := orch.repoutil.HostWorktreePath(multiTask, multiRepoPath, "-be-worktree")
	expectedMultiBePath := filepath.Clean("/tmp/workspaces/task-2/repo-a-be-worktree")
	if filepath.Clean(multiBePath) != expectedMultiBePath {
		t.Errorf("expected multi-repo bePath: %s, got: %s", expectedMultiBePath, multiBePath)
	}

	multiContainerBe := orch.containerPathForHostPath(multiTask, multiBePath, "-be-worktree")
	// Since active step workspace root is multiLocalPath (mounted to /workspace):
	// multiBePath relative to multiLocalPath is "repo-a-be-worktree"
	if multiContainerBe != "/workspace/repo-a-be-worktree" {
		t.Errorf("expected multi-repo container path: /workspace/repo-a-be-worktree, got: %s", multiContainerBe)
	}
}

func TestTestProjectDetectionAndTargetedCommands(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-project-detection-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cases := []struct {
		name       string
		marker     string
		changed    string
		wantKind   orchtester.ProjectKind
		wantInCmd  string
		goPackages map[string]bool
	}{
		{
			name:       "go",
			marker:     "go.mod",
			changed:    "pkg/service.go",
			wantKind:   orchtester.ProjectGo,
			wantInCmd:  "go test -v ./pkg/...",
			goPackages: map[string]bool{"./pkg": true},
		},
		{name: "node", marker: "package.json", changed: "src/app.ts", wantKind: orchtester.ProjectJS, wantInCmd: "npm test"},
		{name: "python", marker: "pyproject.toml", changed: "app/main.py", wantKind: orchtester.ProjectPython, wantInCmd: "pytest"},
		{name: "java", marker: "pom.xml", changed: "src/App.java", wantKind: orchtester.ProjectJava, wantInCmd: "mvn test"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(tmpDir, tc.name)
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(tc.changed)), 0o755); err != nil {
				t.Fatalf("failed to create source dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, tc.marker), []byte("marker"), 0o644); err != nil {
				t.Fatalf("failed to write marker: %v", err)
			}

			kind, markers := orchtester.DetectProjectKindNear(root, tc.changed)
			if kind != tc.wantKind {
				t.Fatalf("expected kind %s, got %s", tc.wantKind, kind)
			}
			if len(markers) == 0 {
				t.Fatal("expected markers for detected project kind")
			}

			goPackages := tc.goPackages
			if goPackages == nil {
				goPackages = map[string]bool{}
			}
			cmd, ok := orchtester.TargetedTestCommand(kind, "/workspace", []string{tc.changed}, goPackages)
			if !ok {
				t.Fatal("expected targeted command")
			}
			if !strings.Contains(cmd, tc.wantInCmd) {
				t.Fatalf("expected command to contain %q, got %q", tc.wantInCmd, cmd)
			}
		})
	}
}

func TestFullVerificationScriptUsesSharedMarkers(t *testing.T) {
	script := orchtester.FullVerificationScript()
	for _, expected := range []string{"go.mod", "package.json", "requirements.txt", "pyproject.toml", "pytest.ini", "pom.xml", "build.gradle"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("expected full verification script to contain marker %s", expected)
		}
	}
}
