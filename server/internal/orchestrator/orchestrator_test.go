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

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
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
	job        *models.WorkflowJob
	checkpoint *models.WorkflowCheckpoint
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
	return nil
}

func (m *mockWorkflowRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	if m.checkpoint != nil {
		return []models.WorkflowCheckpoint{*m.checkpoint}, nil
	}
	return []models.WorkflowCheckpoint{}, nil
}

func (m *mockWorkflowRepo) CreateLog(ctx context.Context, log models.TaskLog) error {
	return nil
}

func (m *mockWorkflowRepo) ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	return []models.TaskLog{}, nil
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
	if len(req.Command) >= 3 && req.Command[2] == "git diff" {
		stdout = "diff --git a/file.go b/file.go\n+new line"
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

type mockLLMProvider struct {
	responses map[string]string
}

func (m *mockLLMProvider) Name() string {
	return "mock-model"
}

func (m *mockLLMProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	lastMsg := messages[len(messages)-1].Content
	content := `{"patch": "diff --git a/main.go b/main.go\n"}`
	for k, v := range m.responses {
		if len(lastMsg) > 0 && (lastMsg == k || lastMsg[len(lastMsg)-1] == k[len(k)-1]) {
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
	committedMsg  string
	prTitle       string
}

func (m *mockGitOpsClient) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	m.clonedRepo = repoURL
	return branch, nil
}

func (m *mockGitOpsClient) CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error) {
	m.clonedRepo = repoURL
	return branch, nil
}

func (m *mockGitOpsClient) CreateBranch(ctx context.Context, repoURL, branchName string) error {
	m.createdBranch = branchName
	return nil
}

func (m *mockGitOpsClient) CommitAndPush(ctx context.Context, repoURL, branchName, message string, files map[string]string, agentRole string) error {
	m.committedMsg = message
	return nil
}

func (m *mockGitOpsClient) CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error) {
	m.prTitle = title
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
	orch := NewOrchestratorWithPrompt(taskRepo, workflowRepo, agentAssigner, sandboxRuntime, nil)
	orch.SetLLMProvider(llmProvider)
	orch.SetGitOpsClient(gitOps)
	orch.SetArtifactRepository(artifactRepo)
	orch.SetRepositoryRepository(reposRepo)
	orch.SetWorkspaceRoot(tmpDir)

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
		if cmd == "git apply patch.diff" || cmd == "git apply --recount --whitespace=nowarn patch.diff" {
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
	if gitOps.createdBranch != "autocode/task-task-123" {
		t.Errorf("expected branch autocode/task-task-123, got %s", gitOps.createdBranch)
	}
	if gitOps.prTitle != "AutoCodeOS: Test Task" {
		t.Errorf("expected PR title, got %s", gitOps.prTitle)
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
			res, err := parseJSONMarkdown(tc.input)
			if err != nil {
				t.Fatalf("parseJSONMarkdown failed: %v", err)
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
	orch := NewOrchestrator(nil, nil, nil, nil)
	orch.SetWorkspaceRoot(root)

	if err := os.MkdirAll(filepath.Join(root, "task-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := orch.removeWorkspace("task-1"); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "task-1")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace removed, got err=%v", err)
	}
	if err := orch.removeWorkspace(""); err == nil {
		t.Fatal("expected empty task id to be rejected")
	}
	if err := orch.removeWorkspace("../outside"); err == nil {
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

	orch := NewOrchestrator(nil, nil, nil, nil)
	orch.SetWorkspaceRoot(root)
	orch.SetWorkspaceRetention(time.Hour, time.Hour)

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

	removed, err := pruneLogFiles(context.Background(), 14, root)
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
