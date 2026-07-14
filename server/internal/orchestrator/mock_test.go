package orchestrator

import (
	"context"
	"errors"
	"strings"

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
	job            *models.WorkflowJob
	checkpoint     *models.WorkflowCheckpoint
	checkpoints    []models.WorkflowCheckpoint
	deletedSteps   []string
	agentUpdateErr error
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
	if updates["agent_id"] != nil && m.agentUpdateErr != nil {
		return nil, m.agentUpdateErr
	}
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
	m.deletedSteps = append([]string{}, steps...)
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

func (m *mockWorkflowRepo) ResetAllRunningJobs(ctx context.Context) error {
	return nil
}

func (m *mockWorkflowRepo) ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	return []models.TaskLog{}, nil
}

func (m *mockWorkflowRepo) TailLogs(ctx context.Context, taskID string, n int) ([]models.TaskLog, error) {
	return []models.TaskLog{}, nil
}

func (m *mockWorkflowRepo) SubscribeLogs(taskID string) chan models.TaskLog {
	return nil
}

func (m *mockWorkflowRepo) UnsubscribeLogs(taskID string, ch chan models.TaskLog) {}

func (m *mockWorkflowRepo) AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error) {
	return "mock-conn", true, nil
}

func (m *mockWorkflowRepo) ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error {
	return nil
}

func (m *mockWorkflowRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	return nil
}

type mockAgentAssigner struct {
	agent       *models.Agent
	releasedIDs []string
}

func (m *mockAgentAssigner) Assign(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.agent, nil
}

func (m *mockAgentAssigner) AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error) {
	a := *m.agent
	a.Role = models.AgentRoleReviewer
	return &a, nil
}

func (m *mockAgentAssigner) AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	a := *m.agent
	a.Role = models.AgentRoleBackend
	return &a, nil
}

func (m *mockAgentAssigner) AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	a := *m.agent
	a.Role = models.AgentRoleFrontend
	return &a, nil
}

func (m *mockAgentAssigner) MarkRunning(ctx context.Context, agentID string) error {
	return nil
}

func (m *mockAgentAssigner) Release(ctx context.Context, agentID string) error {
	m.releasedIDs = append(m.releasedIDs, agentID)
	return nil
}

func (m *mockAgentAssigner) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	if m.agent != nil && m.agent.ID == id {
		return m.agent, nil
	}
	return nil, errors.New("not found")
}

type mockSandboxRuntime struct {
	commands []string
}

func (m *mockSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	stdout := ""
	if len(req.Command) >= 3 && (strings.Contains(req.Command[2], "git diff") || (strings.Contains(req.Command[2], "python3") && strings.Contains(req.Command[2], "diff"))) {
		stdout = "diff --git a/file.go b/file.go\n+new line"
	} else if len(req.Command) >= 3 && (strings.Contains(req.Command[2], "git status --porcelain") || (strings.Contains(req.Command[2], "python3") && strings.Contains(req.Command[2], "status"))) {
		stdout = "code/repos/repo/file.go\n"
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
	responses              map[string]string
	responseQueue          []*llm.Response
	lastChatOptions        llm.ChatOptions
	sawStateMachineEnabled bool
}

func (m *mockLLMProvider) Name() string {
	return "mock-model"
}

func (m *mockLLMProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (m *mockLLMProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	m.lastChatOptions = opts
	if models.IsStateMachineEnabled(ctx) {
		m.sawStateMachineEnabled = true
	}
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

func (m *mockArtifactRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	return nil
}

type mockRepositoriesRepo struct {
	repo models.Repository
}

func (m *mockRepositoriesRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return []models.Repository{m.repo}, nil
}

func (m *mockRepositoriesRepo) ListAll(ctx context.Context) ([]models.Repository, error) {
	return []models.Repository{m.repo}, nil
}
