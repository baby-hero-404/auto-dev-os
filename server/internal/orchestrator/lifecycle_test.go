package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/baby-hero-404/aicli/sandbox"
)

func TestOrchestrator_Resume_DoesNotLoseCLISpec(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch-resume-clispec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:        "task-resume-clispec-123",
		ProjectID: "proj-resume-clispec-123",
	}
	agent := &models.Agent{
		ID:   "agent-123",
		Name: "Test Agent",
	}

	repo := models.Repository{
		ID:        "repo-123",
		ProjectID: "proj-resume-clispec-123",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
	}

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

	workflowRepo := &mockWorkflowRepo{
		job: &models.WorkflowJob{
			ID:     "job-123",
			TaskID: task.ID,
			Status: models.WorkflowJobStatusQueued,
		},
		checkpoint: &models.WorkflowCheckpoint{
			TaskID: task.ID,
			Step:   workflow.StepCLISpec,
			State:  []byte(`{"status": "paused"}`),
		},
	}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithGitOpsClient(gitOps),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(tmpDir),
	)

	// A paused cli_spec checkpoint must be enough to preserve the worktree, even though
	// cli_spec itself hasn't reached "success" yet — ensureWorkspaceCloned must NOT call
	// resetExistingWorkspace (so gitOps must not be queried or clean git commands run).
	err = orch.ensureWorkspaceCloned(context.Background(), task, agent, "job-123")
	if err != nil {
		t.Errorf("expected no error in ensureWorkspaceCloned, got %v", err)
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
	workflowRepo.job = &models.WorkflowJob{
		ID:     "job-paused-merge",
		TaskID: task.ID,
		Status: models.WorkflowJobStatusPaused,
		Step:   models.WorkflowStepPR,
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
	if workflowRepo.job.Status != models.WorkflowJobStatusDone {
		t.Errorf("expected paused workflow job to be marked done after merge approval, got: %s", workflowRepo.job.Status)
	}
}
