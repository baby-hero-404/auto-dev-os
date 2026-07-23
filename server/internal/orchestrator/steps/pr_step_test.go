package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockGitOpsClient struct {
	commitAndPushErr     error
	createPullRequestErr error
	prURL                string
}

func (m *mockGitOpsClient) CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error {
	return m.commitAndPushErr
}

func (m *mockGitOpsClient) CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error) {
	return m.prURL, m.createPullRequestErr
}

type mockArtifactRepository struct {
	artifacts []models.WorkflowArtifact
	err       error
}

func (m *mockArtifactRepository) Create(ctx context.Context, artifact *models.WorkflowArtifact) error {
	return nil
}

func (m *mockArtifactRepository) ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error) {
	return m.artifacts, m.err
}

func (m *mockArtifactRepository) ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error) {
	return m.artifacts, m.err
}

func TestPRStep_ExecutesSuccessfullyAndCreatesPR(t *testing.T) {
	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-1",
		Status:    models.TaskStatusTesting,
	}

	gitMock := &mockSandboxGit{}
	gitopsMock := &mockGitOpsClient{
		prURL: "http://github.com/pr/123",
	}

	worktreeMock := &mockWorktreeManager{
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
		},
	}

	taskRepoMock := &mockTaskRepository{task: task}
	artRepoMock := &mockArtifactRepository{}

	step := NewPRStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		taskRepoMock,
		&mockStatusUpdater{},
		worktreeMock,
		&mockStepWorkspaceLoader{},
		gitMock,
		&mockDiffCapturer{},
		artRepoMock,
		&mockProjectReader{project: &models.Project{ID: "proj-1"}},
		&mockCheckpointLister{},
		gitopsMock,
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/container/path" },
		&mockLogger{},
		nil,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Errorf("expected ErrWaitingApproval, got: %v", err)
	}
}

type mockAttestationSigner struct {
	calls []AttestationSignInput
	err   error
}

func (m *mockAttestationSigner) SignCommit(ctx context.Context, in AttestationSignInput) error {
	m.calls = append(m.calls, in)
	return m.err
}

func TestPRStep_SignsAttestationForCreatedPR(t *testing.T) {
	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-1",
		Status:    models.TaskStatusTesting,
	}

	gitMock := &mockSandboxGit{headCommit: "deadbeef1234"}
	gitopsMock := &mockGitOpsClient{prURL: "http://github.com/pr/123"}
	worktreeMock := &mockWorktreeManager{
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
		},
	}
	signerMock := &mockAttestationSigner{}

	step := NewPRStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskRepository{task: task},
		&mockStatusUpdater{},
		worktreeMock,
		&mockStepWorkspaceLoader{},
		gitMock,
		&mockDiffCapturer{},
		&mockArtifactRepository{},
		&mockProjectReader{project: &models.Project{ID: "proj-1", DefaultAutonomy: "supervised", ReviewHarnessPolicy: models.ReviewHarnessDifferentModel}},
		&mockCheckpointLister{},
		gitopsMock,
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/container/path" },
		&mockLogger{},
		signerMock,
	)

	if _, err := step.Execute(context.Background(), workflow.StepContext{}); !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Fatalf("expected ErrWaitingApproval, got: %v", err)
	}

	if len(signerMock.calls) != 1 {
		t.Fatalf("expected exactly 1 attestation sign call, got %d", len(signerMock.calls))
	}
	call := signerMock.calls[0]
	if call.CommitHash != "deadbeef1234" {
		t.Errorf("expected commit hash 'deadbeef1234', got %q", call.CommitHash)
	}
	if call.TaskID != "task-123" {
		t.Errorf("expected task_id 'task-123', got %q", call.TaskID)
	}
	if call.Autonomy != "supervised" {
		t.Errorf("expected autonomy 'supervised' from project, got %q", call.Autonomy)
	}
	if call.ReviewHarness != models.ReviewHarnessDifferentModel {
		t.Errorf("expected review_harness %q, got %q", models.ReviewHarnessDifferentModel, call.ReviewHarness)
	}
}

func TestPRStep_AttestationSignFailureDoesNotBlockPR(t *testing.T) {
	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-1",
		Status:    models.TaskStatusTesting,
	}

	gitMock := &mockSandboxGit{headCommit: "deadbeef1234"}
	gitopsMock := &mockGitOpsClient{prURL: "http://github.com/pr/123"}
	worktreeMock := &mockWorktreeManager{
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
		},
	}
	signerMock := &mockAttestationSigner{err: errors.New("signing key unavailable")}

	step := NewPRStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskRepository{task: task},
		&mockStatusUpdater{},
		worktreeMock,
		&mockStepWorkspaceLoader{},
		gitMock,
		&mockDiffCapturer{},
		&mockArtifactRepository{},
		&mockProjectReader{project: &models.Project{ID: "proj-1"}},
		&mockCheckpointLister{},
		gitopsMock,
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/container/path" },
		&mockLogger{},
		signerMock,
	)

	if _, err := step.Execute(context.Background(), workflow.StepContext{}); !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Fatalf("expected PR creation to succeed despite attestation failure (fail-soft), got: %v", err)
	}
	if len(signerMock.calls) != 1 {
		t.Fatalf("expected the attestation sign attempt to still happen, got %d calls", len(signerMock.calls))
	}
}
