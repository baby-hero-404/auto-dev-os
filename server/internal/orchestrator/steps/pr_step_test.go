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
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace) {
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
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Errorf("expected ErrWaitingApproval, got: %v", err)
	}
}
