package steps

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockTaskReader struct {
	task *models.Task
	err  error
}

func (m *mockTaskReader) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return m.task, m.err
}

func (m *mockTaskReader) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	if input.Analysis != nil {
		m.task.Analysis = input.Analysis
	}
	if input.SpecStatus != nil {
		m.task.SpecStatus = *input.SpecStatus
	}
	return m.task, m.err
}

type mockStatusUpdater struct {
	called     bool
	lastID     string
	lastStatus string
	err        error
}

func (m *mockStatusUpdater) UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
	m.called = true
	m.lastID = taskID
	m.lastStatus = newStatus
	if m.err != nil {
		return nil, m.err
	}
	return &models.Task{ID: taskID, Status: newStatus}, nil
}

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Log(ctx context.Context, taskID string, jobID *string, level string, message string) {
	m.messages = append(m.messages, message)
}

type mockLLMRunner struct {
	result          StepResult
	err             error
	lastInstruction string
}

func (m *mockLLMRunner) RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (StepResult, error) {
	m.lastInstruction = instruction
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

type mockWorktreeManager struct {
	loadReposFunc  func(ctx context.Context, task *models.Task) ([]models.Repository, error)
	setupBranch    func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool)
	loadReposError error
	setupCalled    bool

	checkpointStepIDs []string // every stepID passed to CreateGitCheckpoint, in order
	checkpointErr     error
	restoredHashes    []string // every commitHash passed to RestoreGitCheckpoint, in order
	restoreErr        error
}

func (m *mockWorktreeManager) LoadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error) {
	if m.loadReposFunc != nil {
		return m.loadReposFunc(ctx, task)
	}
	if m.loadReposError != nil {
		return nil, m.loadReposError
	}
	return []models.Repository{{ID: "repo1", DisplayName: "test-repo"}}, nil
}

func (m *mockWorktreeManager) SetupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
	m.setupCalled = true
	if m.setupBranch != nil {
		m.setupBranch(ctx, task, agent, jobID, repos, ws, skipFE)
	}
}

func (m *mockWorktreeManager) SetupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	return nil
}

func (m *mockWorktreeManager) CommitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	return nil
}

func (m *mockWorktreeManager) ResetRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, worktreeSuffix string) error {
	return nil
}

func (m *mockWorktreeManager) CreateGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	m.checkpointStepIDs = append(m.checkpointStepIDs, stepID)
	if m.checkpointErr != nil {
		return "", m.checkpointErr
	}
	return "mock-commit-hash-" + stepID, nil
}

func (m *mockWorktreeManager) RestoreGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, commitHash string, worktreeSuffix string) error {
	m.restoredHashes = append(m.restoredHashes, commitHash)
	return m.restoreErr
}

func (m *mockWorktreeManager) RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string {
	return "/tmp/test"
}

func (m *mockWorktreeManager) ContainerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string {
	return "/sandbox/test"
}

type mockStepWorkspaceLoader struct {
	loadFunc func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error)
	saveFunc func(task *models.Task, ws *models.TaskWorkspace) error
}

func (m *mockStepWorkspaceLoader) LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, task)
	}
	return &models.TaskWorkspace{Root: "/tmp/ws1"}, nil
}

func (m *mockStepWorkspaceLoader) SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error {
	if m.saveFunc != nil {
		return m.saveFunc(task, ws)
	}
	return nil
}

type mockProjectReader struct {
	project *models.Project
	err     error
}

func (m *mockProjectReader) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return m.project, m.err
}

type mockDiffCapturer struct {
	diffVal             string
	err                 error
	hostPath            string
	changed             []string
	lastWorkspaceSuffix string
	workspaceSuffixes   []string
}

func (m *mockDiffCapturer) CaptureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	m.lastWorkspaceSuffix = worktreeSuffix
	m.workspaceSuffixes = append(m.workspaceSuffixes, worktreeSuffix)
	return m.diffVal, m.err
}

func (m *mockDiffCapturer) CapturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error) {
	return m.diffVal, m.err
}

func (m *mockDiffCapturer) GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	return m.changed, m.err
}

func (m *mockDiffCapturer) GetTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error) {
	return m.hostPath, m.err
}

type mockArtifactSaver struct {
	called  bool
	jobID   string
	taskID  string
	step    string
	artType string
	payload any
}

func (m *mockArtifactSaver) SaveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	m.called = true
	m.jobID = jobID
	m.taskID = taskID
	m.step = step
	m.artType = artType
	m.payload = payload
	return nil
}

type mockReviewerAssigner struct {
	agent       *models.Agent
	err         error
	releasedIDs []string
}

func (m *mockReviewerAssigner) AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.agent, m.err
}

func (m *mockReviewerAssigner) Release(ctx context.Context, agentID string) error {
	m.releasedIDs = append(m.releasedIDs, agentID)
	return nil
}

type mockCheckpointReader struct {
	count int
}

func (m *mockCheckpointReader) CountSuccessful(ctx context.Context, taskID string, step string) int {
	return m.count
}

type mockCheckpointLister struct {
	cps []models.WorkflowCheckpoint
	err error
}

func (m *mockCheckpointLister) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	return m.cps, m.err
}

func (m *mockCheckpointLister) DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error {
	return m.err
}

type mockPatchApplier struct {
	called     bool
	err        error
	validation []error
}

func (m *mockPatchApplier) Validate(ctx context.Context, task *models.Task, patchData string, worktreeSuffix string) []error {
	return m.validation
}

func (m *mockPatchApplier) ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	m.called = true
	return m.err
}

type mockTestRunner struct {
	called bool
	result StepResult
	err    error
}

func (m *mockTestRunner) RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (StepResult, error) {
	m.called = true
	return m.result, m.err
}

type mockLLMChatter struct {
	resp *llm.Response
	err  error
}

func (m *mockLLMChatter) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.resp, m.err
}

func (m *mockLLMChatter) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	return m.resp, m.err
}

type mockSandboxRunner struct {
	result StepResult
	err    error
}

func (m *mockSandboxRunner) RunCommand(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (StepResult, error) {
	return m.result, m.err
}

type mockRepositoryLister struct {
	repos []models.Repository
	err   error
}

func (m *mockRepositoryLister) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return m.repos, m.err
}

type sandboxRunnerAdapter struct {
	run func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error)
}

func (s sandboxRunnerAdapter) RunCommand(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (StepResult, error) {
	return s.run(ctx, task, agent, stepID, command)
}
