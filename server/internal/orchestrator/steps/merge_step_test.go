package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestMergeStep_SkipsOnEasyTask(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityEasy,
	}

	step := NewMergeStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil,
		&mockStepWorkspaceLoader{},
		nil,
		nil,
		nil,
		nil,
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "" },
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

type mockSandboxGit struct {
	checkoutErr error
	hasBeBranch bool
	mergeBeErr  error
	mergeBeStat string
	hasFeBranch bool
	mergeFeErr  error
	mergeFeStat string
	commitErr   error
}

func (m *mockSandboxGit) CheckoutBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error {
	return m.checkoutErr
}
func (m *mockSandboxGit) CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error {
	return nil
}
func (m *mockSandboxGit) HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) bool {
	if branch == "feature/task-123-be" {
		return m.hasBeBranch
	}
	if branch == "feature/task-123-fe" {
		return m.hasFeBranch
	}
	return false
}
func (m *mockSandboxGit) MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) (string, error) {
	if branch == "feature/task-123-be" {
		return m.mergeBeStat, m.mergeBeErr
	}
	if branch == "feature/task-123-fe" {
		return m.mergeFeStat, m.mergeFeErr
	}
	return "", nil
}
func (m *mockSandboxGit) CommitChanges(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, message string) error {
	return m.commitErr
}
func (m *mockSandboxGit) GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error) {
	return nil, nil
}
func (m *mockSandboxGit) GetPRDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error) {
	return "", nil
}

func TestMergeStep_ExecutesSuccessfully(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1"}

	gitMock := &mockSandboxGit{
		hasBeBranch: true,
		hasFeBranch: true,
	}

	worktreeMock := &mockWorktreeManager{
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
		},
	}

	artifactMock := &mockArtifactSaver{}
	statusMock := &mockStatusUpdater{}

	step := NewMergeStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		worktreeMock,
		&mockStepWorkspaceLoader{},
		gitMock,
		&mockDiffCapturer{},
		artifactMock,
		statusMock,
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/container/path" },
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "changes_reconciled" {
		t.Errorf("expected changes_reconciled, got: %v", result["status"])
	}
	if statusMock.lastStatus != models.TaskStatusReviewing {
		t.Errorf("expected status to transition to reviewing, got: %s", statusMock.lastStatus)
	}
}

func TestMergeStep_PauseOnErrorOnConflicts(t *testing.T) {
	task := &models.Task{
		ID:         "task-123",
		Complexity: models.TaskComplexityMedium,
	}
	agent := &models.Agent{ID: "a1"}

	gitMock := &mockSandboxGit{
		hasBeBranch: true,
		mergeBeErr:  errors.New("conflict in code.go"),
		mergeBeStat: models.MergeStatusConflict,
	}

	worktreeMock := &mockWorktreeManager{
		setupBranch: func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
		},
	}

	artifactMock := &mockArtifactSaver{}

	step := NewMergeStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockTaskReader{task: task},
		worktreeMock,
		&mockStepWorkspaceLoader{},
		gitMock,
		&mockDiffCapturer{},
		artifactMock,
		nil,
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/container/path" },
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected pause error, got nil")
	}

	var pauseErr workflow.PauseError
	if !errors.As(err, &pauseErr) {
		t.Errorf("expected workflow.PauseError, got: %v", err)
	}
}
