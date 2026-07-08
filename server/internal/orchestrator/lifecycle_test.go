package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

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
