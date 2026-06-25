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
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockTaskRepo struct {
	task    *models.Task
	updated []models.UpdateTaskInput
}

func (m *mockTaskRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	if m.task != nil && m.task.ID == id {
		return m.task, nil
	}
	return nil, errors.New("not found")
}

func (m *mockTaskRepo) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	m.updated = append(m.updated, input)
	if input.Status != nil {
		m.task.Status = *input.Status
	}
	if input.SpecStatus != nil {
		m.task.SpecStatus = *input.SpecStatus
	}
	if input.Complexity != nil {
		m.task.Complexity = *input.Complexity
	}
	if input.Analysis != nil {
		m.task.Analysis = input.Analysis
	}
	return m.task, nil
}

type mockWorkflowRepo struct {
	job         *models.WorkflowJob
	checkpoint  *models.WorkflowCheckpoint
	checkpoints []models.WorkflowCheckpoint
}

func (m *mockWorkflowRepo) Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	return m.job, nil
}

func (m *mockWorkflowRepo) ClaimNext(ctx context.Context) (*models.WorkflowJob, error) {
	return m.job, nil
}

func (m *mockWorkflowRepo) LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	return m.job, nil
}

func (m *mockWorkflowRepo) UpdateJob(ctx context.Context, id string, updates map[string]any) (*models.WorkflowJob, error) {
	if updates["status"] != nil {
		m.job.Status = updates["status"].(string)
	}
	if updates["agent_id"] != nil {
		agentID := updates["agent_id"].(string)
		m.job.AgentID = &agentID
	}
	return m.job, nil
}

func (m *mockWorkflowRepo) CreateCheckpoint(ctx context.Context, cp models.WorkflowCheckpoint) error {
	m.checkpoint = &cp
	m.checkpoints = append(m.checkpoints, cp)
	return nil
}

func (m *mockWorkflowRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	if len(m.checkpoints) > 0 {
		return m.checkpoints, nil
	}
	if m.checkpoint != nil {
		return []models.WorkflowCheckpoint{*m.checkpoint}, nil
	}
	return []models.WorkflowCheckpoint{}, nil
}

func (m *mockWorkflowRepo) DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error {
	m.checkpoint = nil
	m.checkpoints = nil
	return nil
}

func (m *mockWorkflowRepo) CreateLog(ctx context.Context, log models.TaskLog) error {
	return nil
}

func (m *mockWorkflowRepo) ResetStuckJobs(ctx context.Context) error {
	return nil
}

func (m *mockWorkflowRepo) ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	return []models.TaskLog{}, nil
}

func (m *mockWorkflowRepo) AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error) {
	return "mock-conn", true, nil
}

func (m *mockWorkflowRepo) ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error {
	return nil
}

type mockAgentAssigner struct {
	agent *models.Agent
}

func (m *mockAgentAssigner) Assign(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.agent, nil
}

func (m *mockAgentAssigner) AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.agent, nil
}

func (m *mockAgentAssigner) AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.agent, nil
}

func (m *mockAgentAssigner) AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.agent, nil
}

func (m *mockAgentAssigner) MarkRunning(ctx context.Context, agentID string) error {
	return nil
}

func (m *mockAgentAssigner) Release(ctx context.Context, agentID string) error {
	return nil
}

type mockSandboxRuntime struct {
	commands []string
}

func (m *mockSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	stdout := ""
	if len(req.Command) >= 3 && strings.Contains(req.Command[2], "git diff") {
		stdout = "diff --git a/file.go b/file.go\n+new line"
	} else if len(req.Command) >= 3 && strings.Contains(req.Command[2], "git status --porcelain") {
		stdout = " M file.go\n"
	}
	return &sandbox.CommandResult{
		ExitCode: 0,
		Stdout:   stdout,
	}, nil
}

func (m *mockSandboxRuntime) Health(ctx context.Context) error {
	return nil
}

func (m *mockSandboxRuntime) Prewarm(ctx context.Context) error {
	return nil
}

type mockAnalyzeSandboxRuntime struct {
	commands []string
	outputs  map[string]string
}

func (m *mockAnalyzeSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	cmd := ""
	if len(req.Command) >= 3 {
		cmd = req.Command[2]
	}
	m.commands = append(m.commands, cmd)
	for contains, out := range m.outputs {
		if strings.Contains(cmd, contains) {
			return &sandbox.CommandResult{ExitCode: 0, Stdout: out}, nil
		}
	}
	return &sandbox.CommandResult{ExitCode: 0, Stdout: ""}, nil
}

func (m *mockAnalyzeSandboxRuntime) Health(ctx context.Context) error {
	return nil
}

func (m *mockAnalyzeSandboxRuntime) Prewarm(ctx context.Context) error {
	return nil
}

type mockLLMProvider struct {
	responses       map[string]string
	responseQueue   []*llm.Response
	lastChatOptions llm.ChatOptions
}

func (m *mockLLMProvider) Name() string {
	return "mock-model"
}

func (m *mockLLMProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (m *mockLLMProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	m.lastChatOptions = opts
	if len(m.responseQueue) > 0 {
		resp := m.responseQueue[0]
		m.responseQueue = m.responseQueue[1:]
		return resp, nil
	}
	lastMsg := messages[len(messages)-1].Content
	content := `{"patch": "diff --git a/main.go b/main.go\n"}`
	for k, v := range m.responses {
		if strings.Contains(lastMsg, k) {
			content = v
			break
		}
	}
	return &llm.Response{
		Model:        "mock-model",
		Content:      content,
		PromptTokens: 10,
		OutputTokens: 20,
	}, nil
}

type mockGitOpsClient struct {
	clonedRepo    string
	createdBranch string

	committedFiles int
	prTitle        string
	prURLSet       bool
	prURL          string
}

func (m *mockGitOpsClient) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	m.clonedRepo = repoURL
	return branch, nil
}

func (m *mockGitOpsClient) CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error) {
	m.clonedRepo = repoURL
	return branch, nil
}

func (m *mockGitOpsClient) CreateBranch(ctx context.Context, localPath, repoURL, branchName string) error {
	m.createdBranch = branchName
	return nil
}

func (m *mockGitOpsClient) CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error {
	m.createdBranch = branchName
	m.committedFiles = len(files)
	return nil
}

func (m *mockGitOpsClient) CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error) {
	m.prTitle = title
	if m.prURLSet {
		return m.prURL, nil
	}
	return "https://github.com/mock/pr/1", nil
}

func (m *mockGitOpsClient) MergePullRequest(ctx context.Context, repoURL, prURL string) error {
	return nil
}

type mockArtifactRepo struct {
	artifacts []models.WorkflowArtifact
}

func (m *mockArtifactRepo) Create(ctx context.Context, artifact *models.WorkflowArtifact) error {
	m.artifacts = append(m.artifacts, *artifact)
	return nil
}

func (m *mockArtifactRepo) ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error) {
	return m.artifacts, nil
}

func (m *mockArtifactRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error) {
	return m.artifacts, nil
}

type mockRepositoriesRepo struct {
	repo models.Repository
}

func (m *mockRepositoriesRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return []models.Repository{m.repo}, nil
}

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

	// 2. Verify git apply and git diff command executions in sandbox
	appliedPatch := false
	capturedDiff := false
	for _, cmd := range sandboxRuntime.commands {
		if strings.Contains(cmd, "patch.diff") {
			appliedPatch = true
		}
		if cmd == "git diff" || strings.Contains(cmd, "git diff") {
			capturedDiff = true
		}
	}
	if !appliedPatch {
		t.Error("expected 'git apply patch.diff' to have been executed in sandbox")
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

	// 4. Verify artifacts were saved in DB
	expectedTypes := map[string]bool{
		"prompt":          false,
		"llm_response":    false,
		"patch":           false,
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

	out, err := runner(context.Background(), workflow.StepContext{Inputs: map[string]map[string]any{
		workflow.StepTest: map[string]any{"status": "passed"},
	}})
	if err != nil {
		t.Fatalf("PR step failed: %v", err)
	}
	if status, _ := out["status"].(string); status != "no_changes_detected" {
		t.Fatalf("expected no_changes_detected status, got %v", out["status"])
	}
	if task.Status != models.TaskStatusMerged {
		t.Fatalf("expected task status merged, got %s", task.Status)
	}
	if len(taskRepo.updated) == 0 {
		t.Fatal("expected task updates to be recorded")
	}
	for _, update := range taskRepo.updated {
		if update.PRURLs != nil {
			t.Fatalf("expected no PR URL update when no changes are detected, got %v", []string(*update.PRURLs))
		}
		if update.Status != nil && *update.Status != models.TaskStatusMerged {
			t.Fatalf("expected only merged status update, got %s", *update.Status)
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

	if err := os.MkdirAll(filepath.Join(root, "task-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := orch.RemoveWorkspace("task-1"); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "task-1")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace removed, got err=%v", err)
	}
	if err := orch.RemoveWorkspace(""); err == nil {
		t.Fatal("expected empty task id to be rejected")
	}
	if err := orch.RemoveWorkspace("../outside"); err == nil {
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

	removed, err := orch.pruneWorkspaces(context.Background())
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

func TestOrchestrator_AutonomyLevel_StepAnalyze(t *testing.T) {
	task := &models.Task{
		ID:          "task-1",
		ProjectID:   "proj-1",
		Title:       "Test Task",
		Description: "Simple test description",
		Complexity:  models.TaskComplexityMedium,
		SpecStatus:  models.TaskSpecStatusDraft,
	}

	agent := &models.Agent{
		ID:            "agent-1",
		Name:          "Agent 1",
		AutonomyLevel: models.AgentAutonomyAutonomous,
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: &models.WorkflowJob{ID: "job-1"}}
	llmResponses := map[string]any{
		"complexity":              "medium",
		"spec_status":             "approved",
		"clarification_questions": []string{},
		"affected_files":          []string{},
		"execution_plan":          []string{},
		"system_prompt":           "mock",
	}
	rawBytes, _ := json.Marshal(llmResponses)
	mockLLM := &mockLLMProvider{responses: map[string]string{
		"Analyze": string(rawBytes),
	}}
	orch := New(taskRepo, workflowRepo, nil, nil, WithLLMProvider(mockLLM))

	runners := orch.stepRunners(task, agent, "job-1", "")
	analyzeRunner := runners[workflow.StepAnalyze]

	res, err := analyzeRunner(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error for autonomous agent: %v", err)
	}

	if task.SpecStatus != models.TaskSpecStatusAutoApproved {
		t.Errorf("expected spec status AutoApproved, got %s", task.SpecStatus)
	}
	if task.Status != models.TaskStatusCoding {
		t.Errorf("expected task status Coding, got %s", task.Status)
	}
	if res["spec_status"] != models.TaskSpecStatusAutoApproved {
		t.Errorf("expected output spec_status AutoApproved, got %v", res["spec_status"])
	}
}

func TestOrchestrator_StepAnalyze_UsesNativeToolCalls(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "native-analyze-tools-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:          "task-native-tools",
		ProjectID:   "proj-native-tools",
		Title:       "Inspect code",
		Description: "Analyze the current source before planning",
		Status:      models.TaskStatusAnalyzing,
		Complexity:  models.TaskComplexityMedium,
		SpecStatus:  models.TaskSpecStatusDraft,
	}
	agent := &models.Agent{
		ID:            "agent-native-tools",
		Name:          "Agent Native Tools",
		AutonomyLevel: models.AgentAutonomyAutonomous,
	}
	finalAnalysis := map[string]any{
		"complexity":              "medium",
		"primary_category":        "backend",
		"scope":                   "Inspect code and prepare implementation plan.",
		"affected_files":          []string{"src/main.go"},
		"risks":                   []string{},
		"risk_domains":            []string{},
		"execution_plan":          []string{"Read source files", "Implement changes"},
		"clarification_questions": []string{},
		"required_skills":         []string{},
		"proposal_md":             "## Why\nNeed change.\n",
		"specs_md":                "## ADDED Requirements\n### Requirement: Native tools\nThe analyzer SHALL use native tools.\n",
		"design_md":               "## Context\nNative tools.\n",
		"tasks_md":                "## 1. Work\n- [ ] 1.1 Implement\n",
	}
	finalBytes, _ := json.Marshal(finalAnalysis)
	mockLLM := &mockLLMProvider{responseQueue: []*llm.Response{
		{
			Model: "mock-model",
			ToolCalls: []llm.ToolCall{{
				ID:        "call-list-files",
				Name:      "list_files",
				Arguments: "{}",
			}},
		},
		{
			Model:   "mock-model",
			Content: string(finalBytes),
		},
	}}
	analyzeRuntime := &mockAnalyzeSandboxRuntime{outputs: map[string]string{
		"find .": "src/main.go\n",
	}}
	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: &models.WorkflowJob{ID: "job-native-tools"}}
	orch := New(taskRepo, workflowRepo, nil, analyzeRuntime,
		WithWorkspaceRoot(tmpDir),
		WithLLMProvider(mockLLM),
	)

	runners := orch.stepRunners(task, agent, "job-native-tools", "")
	res, err := runners[workflow.StepAnalyze](context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error for native tool analyze: %v", err)
	}
	if res["spec_status"] != models.TaskSpecStatusAutoApproved {
		t.Fatalf("expected auto-approved spec, got %v", res["spec_status"])
	}
	if len(mockLLM.lastChatOptions.Tools) != 3 {
		t.Fatalf("expected analyze tool definitions to be passed to LLM, got %+v", mockLLM.lastChatOptions.Tools)
	}
	if len(analyzeRuntime.commands) == 0 || !strings.Contains(strings.Join(analyzeRuntime.commands, "\n"), "find .") {
		t.Fatalf("expected list_files native tool to execute through sandbox, got %#v", analyzeRuntime.commands)
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
		"fix": `{"patch": "diff --git a/main.go b/main.go\n+fix", "fixes_applied": true}`,
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
					"findings": []any{"finding 1"},
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
					"findings": []any{"finding 1"},
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

	out, err := steps.ExecuteReview(context.Background(), orch.makeStepsDeps(task, agent, "job-review-cycle"), task, agent, "job-review-cycle", workflow.StepContext{})
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

	ws := orch.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: "repo-b",
		Name:   "repo-b",
		Paths:  models.RepoWorkspacePaths{Main: filepath.Join("code", "repos", "repo-b", "main")},
	}}
	if err := os.MkdirAll(ws.Root, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	if err := orch.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	if _, err := orch.getTaskRepoHostPath(context.Background(), task); err == nil {
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
	ws := orch.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: repoID,
		Name:   "repo-a",
		Paths:  models.RepoWorkspacePaths{Main: filepath.Join("code", "repos", "repo-a", "main")},
	}}
	repoRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
	if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := orch.SaveTaskWorkspaceMetadata(task, ws); err != nil {
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

func TestOrchestrator_Resume_DoesNotLoseCode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch-resume-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:        "task-resume-123",
		ProjectID: "proj-resume-123",
	}
	agent := &models.Agent{
		ID:   "agent-123",
		Name: "Test Agent",
	}

	repo := models.Repository{
		ID:        "repo-123",
		ProjectID: "proj-resume-123",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
	}

	// Mock workspace state with .git so it looks existing
	localPath := sandbox.WorkspacePath(tmpDir, task.ID)
	gitDir := filepath.Join(localPath, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("failed to setup mock git dir: %v", err)
	}

	taskRepo := &mockTaskRepo{task: task}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

	// Set up workflow repo with a successful checkpoint for code_backend
	workflowRepo := &mockWorkflowRepo{
		job: &models.WorkflowJob{
			ID:     "job-123",
			TaskID: task.ID,
			Status: models.WorkflowJobStatusQueued,
		},
		checkpoint: &models.WorkflowCheckpoint{
			TaskID: task.ID,
			Step:   workflow.StepCodeBackend,
			State:  []byte(`{"status": "success"}`),
		},
	}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithGitOpsClient(gitOps),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(tmpDir),
	)

	// Since hasSuccessfulCodeStep is true and local git directory exists,
	// ensureWorkspaceCloned must NOT call resetExistingWorkspace (so gitOps must not be queried or clean git commands run).
	err = orch.ensureWorkspaceCloned(context.Background(), task, agent, "job-123")
	if err != nil {
		t.Errorf("expected no error in ensureWorkspaceCloned, got %v", err)
	}
}

func TestOrchestrator_Fix_WithPRRejection(t *testing.T) {
	task := &models.Task{
		ID:         "task-fix-123",
		Complexity: models.TaskComplexityMedium,
		Status:     models.TaskStatusCoding,
	}
	agent := &models.Agent{
		ID: "agent-123",
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{
		job: &models.WorkflowJob{
			ID:     "job-123",
			TaskID: task.ID,
		},
		// Return pr_rejection feedback in checkpoints
		checkpoint: &models.WorkflowCheckpoint{
			TaskID: task.ID,
			Step:   "pr_rejection",
			State:  []byte(`{"feedback": "please fix database naming"}`),
		},
	}

	llmResponses := map[string]string{
		"fix": `{"patch": "diff --git a/main.go b/main.go\n+fix", "fixes_applied": true}`,
	}
	mockLLM := &mockLLMProvider{responses: llmResponses}
	sandboxRuntime := &mockSandboxRuntime{}

	orch := New(taskRepo, workflowRepo, nil, sandboxRuntime, WithLLMProvider(mockLLM))

	runners := orch.stepRunners(task, agent, "job-loop", "")
	fixRunner := runners[workflow.StepFix]

	ctx := context.Background()
	stepCtx := workflow.StepContext{
		Inputs: map[string]map[string]any{
			workflow.StepReview: {
				"cycle_limit_reached": false,
				"parsed": map[string]any{
					// Zero findings from AI review
					"findings": []any{},
				},
			},
		},
	}

	// Since we have pr_rejection feedback in checkpoints, the fix runner must NOT skip
	// the fix step (which it normally would because of findings = 0), and must instead return ErrReviewFixLoop
	res, err := fixRunner(ctx, stepCtx)
	if !errors.Is(err, workflow.ErrReviewFixLoop) {
		t.Errorf("expected ErrReviewFixLoop even with 0 review findings due to human feedback, got err=%v, res=%v", err, res)
	}
}

func TestOrchestrator_ApproveMerge(t *testing.T) {
	task := &models.Task{
		ID:        "task-merge-123",
		ProjectID: "proj-merge-123",
		Status:    models.TaskStatusHumanReview,
		PRURLs:    []string{"https://github.com/test/repo/pull/1"},
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{}
	gitOps := &mockGitOpsClient{}

	// Case 1: No matching repository found (should return error)
	reposRepoNoMatch := &mockRepositoriesRepo{
		repo: models.Repository{
			URL: "https://github.com/different/repo.git",
		},
	}
	orchNoMatch := New(taskRepo, workflowRepo, nil, nil,
		WithGitOpsClient(gitOps),
		WithRepositoryRepository(reposRepoNoMatch),
	)

	_, err := orchNoMatch.ApproveMerge(context.Background(), task.ID)
	if err == nil || !strings.Contains(err.Error(), "no matching repository found for PR URL") {
		t.Errorf("expected unmatched repository error, got: %v", err)
	}

	// Case 2: Matching repository found (should succeed)
	reposRepoMatch := &mockRepositoriesRepo{
		repo: models.Repository{
			URL: "https://github.com/test/repo.git",
		},
	}
	orchMatch := New(taskRepo, workflowRepo, nil, nil,
		WithGitOpsClient(gitOps),
		WithRepositoryRepository(reposRepoMatch),
	)

	updated, err := orchMatch.ApproveMerge(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("expected successful merge, got error: %v", err)
	}
	if updated.Status != models.TaskStatusMerged {
		t.Errorf("expected task status to transition to merged, got: %s", updated.Status)
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
	if !strings.Contains(splitPatches["repo1"], "repo1/src/main.go") {
		t.Errorf("expected repo1 patch to contain main.go, got: %s", splitPatches["repo1"])
	}
	if strings.Contains(splitPatches["repo1"], "repo2/index.js") {
		t.Errorf("repo1 patch should not contain repo2 changes")
	}
	if !strings.Contains(splitPatches["repo2"], "repo2/index.js") {
		t.Errorf("expected repo2 patch to contain index.js, got: %s", splitPatches["repo2"])
	}

	// 2. Test hostWorktreePath and containerPathForHostPath
	orch := &Orchestrator{
		workspaceRoot: "/tmp/workspaces",
	}

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

	repoPathWithoutMetadata := orch.repoHostPath(singleTask, nil, repo)
	expectedRepoPathWithoutMetadata := filepath.Clean("/tmp/workspaces/task-1/code/repos/repo-a/main")
	if filepath.Clean(repoPathWithoutMetadata) != expectedRepoPathWithoutMetadata {
		t.Errorf("expected repo path without metadata: %s, got: %s", expectedRepoPathWithoutMetadata, repoPathWithoutMetadata)
	}

	// Suffix case
	bePath := orch.hostWorktreePath(singleTask, localPath, "-be-worktree")
	expectedBePath := filepath.Clean("/tmp/workspaces/task-1/be")
	if filepath.Clean(bePath) != expectedBePath {
		t.Errorf("expected single-repo bePath: %s, got: %s", expectedBePath, bePath)
	}

	containerBe := orch.containerPathForHostPath(singleTask, bePath, "-be-worktree")
	if containerBe != "/workspace" {
		t.Errorf("expected containerPath for worktree to be /workspace, got: %s", containerBe)
	}

	// Child in worktree case
	containerBeFile := orch.containerPathForHostPath(singleTask, filepath.Join(bePath, "src/main.go"), "-be-worktree")
	if containerBeFile != "/workspace/src/main.go" {
		t.Errorf("expected container path /workspace/src/main.go, got: %s", containerBeFile)
	}

	// Multi repo case
	multiTask := &models.Task{
		ID:           "task-2",
		RepositoryID: nil,
	}

	multiLocalPath := "/tmp/workspaces/task-2"
	multiRepoPath := filepath.Join(multiLocalPath, "repo-a")

	multiBePath := orch.hostWorktreePath(multiTask, multiRepoPath, "-be-worktree")
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

func TestOrchestrator_StepAnalyze_FallbackForcesReview(t *testing.T) {
	task := &models.Task{
		ID:          "task-fallback",
		ProjectID:   "proj-fallback",
		Title:       "Test Fallback Task",
		Description: "Simple test description",
		Status:      models.TaskStatusAnalyzing,
		Complexity:  models.TaskComplexityMedium,
		SpecStatus:  models.TaskSpecStatusDraft,
	}

	agent := &models.Agent{
		ID:            "agent-fallback",
		Name:          "Agent Fallback",
		AutonomyLevel: models.AgentAutonomyAutonomous, // autonomous would normally auto-approve
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: &models.WorkflowJob{ID: "job-fallback"}}
	// Malformed LLM response to trigger fallback
	mockLLM := &mockLLMProvider{responses: map[string]string{
		"": "this is not valid JSON and will fail parsing",
	}}
	orch := New(taskRepo, workflowRepo, nil, nil, WithLLMProvider(mockLLM))

	runners := orch.stepRunners(task, agent, "job-fallback", "")
	analyzeRunner := runners[workflow.StepAnalyze]

	res, err := analyzeRunner(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatalf("expected PauseError, got nil")
	}

	var pauseErr workflow.PauseError
	if !errors.As(err, &pauseErr) {
		t.Fatalf("expected workflow.PauseError, got %v", err)
	}

	if pauseErr.Step != workflow.StepAnalyze {
		t.Errorf("expected PauseError step %s, got %s", workflow.StepAnalyze, pauseErr.Step)
	}

	if !strings.Contains(pauseErr.Reason, "fallback from malformed analyzer output") {
		t.Errorf("expected PauseError reason to mention fallback, got: %s", pauseErr.Reason)
	}

	if task.SpecStatus != models.TaskSpecStatusPendingReview {
		t.Errorf("expected spec status PendingReview, got %s", task.SpecStatus)
	}
	if task.Status != models.TaskStatusSpecReview {
		t.Errorf("expected task status SpecReview, got %s", task.Status)
	}
	if res != nil {
		t.Errorf("expected nil result on pause error, got %v", res)
	}
}

func TestOrchestrator_StepAnalyze_ComplexityChangeTriggersGraphRebuild(t *testing.T) {
	task := &models.Task{
		ID:         "task-graph-change",
		ProjectID:  "proj-graph-change",
		Status:     models.TaskStatusAnalyzing,
		Complexity: models.TaskComplexityEasy, // Initial complexity
		SpecStatus: models.TaskSpecStatusDraft,
	}

	agent := &models.Agent{
		ID:            "agent-graph-change",
		AutonomyLevel: models.AgentAutonomyAutonomous,
	}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: &models.WorkflowJob{ID: "job-graph-change"}}
	mockLLM := &mockLLMProvider{responses: map[string]string{
		"": `{
			"complexity": "hard",
			"primary_category": "backend",
			"scope": "test scope",
			"affected_files": [],
			"risks": [],
			"risk_domains": [],
			"execution_plan": [],
			"clarification_questions": [],
			"required_skills": []
		}`,
	}}
	orch := New(taskRepo, workflowRepo, nil, nil, WithLLMProvider(mockLLM))

	runners := orch.stepRunners(task, agent, "job-graph-change", "")
	analyzeRunner := runners[workflow.StepAnalyze]

	res, err := analyzeRunner(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatalf("expected ErrGraphChanged, got nil")
	}

	if !errors.Is(err, workflow.ErrGraphChanged) {
		t.Fatalf("expected workflow.ErrGraphChanged, got %v", err)
	}

	if task.Complexity != models.TaskComplexityHard {
		t.Errorf("expected complexity to update to hard, got %s", task.Complexity)
	}
	if task.SpecStatus != models.TaskSpecStatusAutoApproved {
		t.Errorf("expected auto_approved spec status, got %s", task.SpecStatus)
	}
	if res == nil {
		t.Errorf("expected result to contain output even when returning ErrGraphChanged")
	}
}
