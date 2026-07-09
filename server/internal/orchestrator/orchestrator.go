package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/checkpoint"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/repoutil"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// Orchestrator coordinates the end-to-end workflow for task execution:
// agent assignment, workspace provisioning, step execution, and cleanup.
type Orchestrator struct {
	tasks           TaskRepository
	workflows       WorkflowRepository
	agents          AgentAssigner
	runtime         sandbox.Runtime
	prompts         PromptBuilder
	llm             llm.Provider
	ctxEngine       provider.ContextEngine
	memHooks        MemoryRecorder
	learnEngine     LearningRecorder
	gitOps          GitOpsClient
	artifacts       ArtifactRepository
	repositories    RepositoryRepository
	projects        ProjectRepository
	sandboxGit      gitops.SandboxGitClient
	workspaceRoot   string
	dataRoot        string
	retention       WorkspaceRetention
	wg              sync.WaitGroup
	lockCancels     sync.Map
	lockConns       sync.Map
	jobCancels      sync.Map
	wkspace         *wkspace.Manager
	checkpoints     *checkpoint.Store
	repoutil        *repoutil.Manager
	llmTraceEnabled bool
	llmLogLevel     string
	maxPhaseCost    float64
	wakeChan        chan struct{}
}

func (o *Orchestrator) wake() {
	if o.wakeChan == nil {
		return
	}
	select {
	case o.wakeChan <- struct{}{}:
	default:
	}
}

// WorkspaceRetention configures how long completed workspaces are kept.
type WorkspaceRetention struct {
	Retention time.Duration
	Interval  time.Duration
}

func defaultWorkspaceRetention() WorkspaceRetention {
	return WorkspaceRetention{Retention: 72 * time.Hour, Interval: time.Hour}
}

type Option func(*Orchestrator)

func WithMemoryHooks(hooks MemoryRecorder) Option {
	return func(o *Orchestrator) {
		o.memHooks = hooks
	}
}

func WithLearningEngine(engine LearningRecorder) Option {
	return func(o *Orchestrator) {
		o.learnEngine = engine
	}
}

func WithGitOpsClient(client GitOpsClient) Option {
	return func(o *Orchestrator) {
		o.gitOps = client
	}
}

func WithArtifactRepository(repo ArtifactRepository) Option {
	return func(o *Orchestrator) {
		o.artifacts = repo
	}
}

func WithRepositoryRepository(repo RepositoryRepository) Option {
	return func(o *Orchestrator) {
		o.repositories = repo
	}
}

func WithProjectRepository(repo ProjectRepository) Option {
	return func(o *Orchestrator) {
		o.projects = repo
	}
}

func WithWorkspaceRoot(rootPath string) Option {
	return func(o *Orchestrator) {
		o.workspaceRoot = rootPath
	}
}

func WithDataRoot(dataRoot string) Option {
	return func(o *Orchestrator) {
		o.dataRoot = dataRoot
	}
}

func WithLLMTraceLogging(enabled bool, logLevel string) Option {
	return func(o *Orchestrator) {
		o.llmTraceEnabled = enabled
		o.llmLogLevel = logLevel
	}
}

func WithWorkspaceRetention(retention, interval time.Duration) Option {
	return func(o *Orchestrator) {
		o.retention = WorkspaceRetention{Retention: retention, Interval: interval}
	}
}

func WithMaxPhaseCost(cost float64) Option {
	return func(o *Orchestrator) {
		o.maxPhaseCost = cost
	}
}

func WithLLMProvider(provider llm.Provider) Option {
	return func(o *Orchestrator) {
		o.llm = provider
	}
}

func WithContextEngine(engine provider.ContextEngine) Option {
	return func(o *Orchestrator) {
		o.ctxEngine = engine
	}
}

func WithPrompts(prompts PromptBuilder) Option {
	return func(o *Orchestrator) {
		o.prompts = prompts
	}
}

func New(taskRepo TaskRepository, workflowRepo WorkflowRepository, agentManager AgentAssigner, runtime sandbox.Runtime, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		tasks:           taskRepo,
		workflows:       workflowRepo,
		agents:          agentManager,
		runtime:         runtime,
		retention:       defaultWorkspaceRetention(),
		llmTraceEnabled: true,
		llmLogLevel:     "debug",
		wakeChan:        make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(o)
	}
	o.sandboxGit = gitops.NewSandboxGitClient(o.runSandboxStep, o.log)
	return o
}

func (o *Orchestrator) ListArtifacts(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error) {
	if o.artifacts == nil {
		return nil, fmt.Errorf("artifact repository not configured")
	}
	return o.artifacts.ListByJobID(ctx, jobID)
}

func (o *Orchestrator) Execute(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Avoid duplicate/concurrent execution if a job is already queued or running
	if job, err := o.workflows.LatestByTaskID(ctx, taskID); err == nil && job != nil {
		if job.Status == models.WorkflowJobStatusRunning || job.Status == models.WorkflowJobStatusQueued {
			return job, nil
		}
	}

	// Check if there is a paused job to resume
	if job, err := o.workflows.LatestByTaskID(ctx, taskID); err == nil && job.Status == models.WorkflowJobStatusPaused {
		updated, err := o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusQueued})
		if err != nil {
			return nil, err
		}
		if task.Status == models.TaskStatusTodo || task.Status == models.TaskStatusFailed || task.Status == "" {
			if _, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusContextLoading); err != nil {
				return nil, err
			}
		}
		o.log(ctx, taskID, &job.ID, "info", "paused workflow job resumed")
		o.wake()
		return updated, nil
	}

	if task.Status == models.TaskStatusTodo || task.Status == models.TaskStatusFailed || task.Status == "" {
		if _, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusContextLoading); err != nil {
			return nil, err
		}
	}

	job, err := o.workflows.Enqueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, &job.ID, "info", "workflow job queued")
	o.wake()
	return job, nil
}

// RetryFromLastStep re-enqueues a failed task for execution.
// The worker will automatically load checkpoints and skip steps
// that already succeeded, resuming from the first incomplete step.
func (o *Orchestrator) RetryFromLastStep(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != models.TaskStatusFailed && task.Status != models.TaskStatusSpecReview && task.Status != models.TaskStatusAnalyzing {
		return nil, fmt.Errorf("can only retry paused or failed tasks (current status: %s)", task.Status)
	}

	// 1. If there is an active/paused job, cancel and supersede it, releasing locks.
	if job, err := o.workflows.LatestByTaskID(ctx, taskID); err == nil && job != nil {
		if job.Status == models.WorkflowJobStatusRunning || job.Status == models.WorkflowJobStatusQueued || job.Status == models.WorkflowJobStatusPaused {
			if cancelVal, ok := o.jobCancels.Load(job.ID); ok {
				if cancel, ok := cancelVal.(context.CancelFunc); ok {
					cancel()
				}
			}
			_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
				"status":     models.WorkflowJobStatusFailed,
				"last_error": "superseded by retry",
			})
			o.releaseWorkspaceLock(taskID)
		}
	}

	// Transition task back to analyzing so the workflow can start.
	if _, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusAnalyzing); err != nil {
		return nil, fmt.Errorf("failed to transition task for retry: %w", err)
	}

	job, err := o.workflows.Enqueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, &job.ID, "info", "workflow retried from last successful checkpoint")
	o.wake()
	return job, nil
}

// SavePRRejectionFeedback saves human PR rejection feedback as a task checkpoint.
func (o *Orchestrator) SavePRRejectionFeedback(ctx context.Context, taskID string, feedback string) error {
	stateBytes, err := json.Marshal(map[string]any{"feedback": feedback})
	if err != nil {
		return err
	}
	return o.workflows.CreateCheckpoint(ctx, models.WorkflowCheckpoint{
		TaskID: taskID,
		Step:   "pr_rejection",
		State:  stateBytes,
	})
}

// ClearCheckpointsForRepair clears downstream checkpoints for repair on PR rejection.
func (o *Orchestrator) ClearCheckpointsForRepair(ctx context.Context, taskID string) error {
	steps := []string{"code_backend", "code_frontend", "review", "fix", "test", "pr"}
	if o.tasks != nil {
		if _, err := o.tasks.GetByID(ctx, taskID); err != nil {
			return err
		}
	}
	return o.workflows.DeleteCheckpoints(ctx, taskID, steps)
}

func (o *Orchestrator) WorkflowStatus(ctx context.Context, taskID string) (*models.WorkflowStatus, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, err
	}
	job, _ := o.workflows.LatestByTaskID(ctx, taskID)
	return &models.WorkflowStatus{Task: task, Job: job, Checkpoints: checkpoints}, nil
}

func (o *Orchestrator) Logs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	return o.workflows.ListLogs(ctx, taskID)
}

func (o *Orchestrator) ApproveMerge(ctx context.Context, taskID string) (*models.Task, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != models.TaskStatusHumanReview && task.Status != models.TaskStatusPrReady {
		return nil, fmt.Errorf("task is not waiting for human PR approval")
	}
	if len(task.PRURLs) > 0 && o.gitOps != nil {
		repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to list project repositories: %w", err)
		}
		for _, prURL := range task.PRURLs {
			var matchRepo string
			for _, r := range repos {
				baseRepo := strings.TrimSuffix(r.URL, ".git")
				if strings.Contains(prURL, baseRepo) {
					matchRepo = r.URL
					break
				}
			}
			if matchRepo == "" {
				return nil, fmt.Errorf("no matching repository found for PR URL: %s", prURL)
			}
			if err := o.gitOps.MergePullRequest(ctx, matchRepo, prURL); err != nil {
				o.log(ctx, task.ID, nil, "error", fmt.Sprintf("failed to merge PR %s: %v", prURL, err))
				return nil, fmt.Errorf("failed to merge PR %s: %w", prURL, err)
			} else {
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("successfully merged PR: %s", prURL))
			}
		}
	}

	updated, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusMerged)
	if err != nil {
		return nil, err
	}
	if job, err := o.workflows.LatestByTaskID(ctx, taskID); err == nil && job != nil && job.Status == models.WorkflowJobStatusPaused {
		_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
			"status":     models.WorkflowJobStatusDone,
			"step":       models.WorkflowStepDone,
			"last_error": "",
		})
		_ = o.checkpoint(ctx, taskID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone})
	}
	o.log(ctx, taskID, nil, "info", "human approved workflow for merge")
	return updated, nil
}

func (o *Orchestrator) StartReview(ctx context.Context, taskID string) (*models.Task, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != models.TaskStatusPrReady {
		return nil, fmt.Errorf("task is not in pr_ready state")
	}
	return o.updateTaskStatus(ctx, taskID, models.TaskStatusHumanReview)
}

func (o *Orchestrator) CheckReviewLoopLimit(ctx context.Context, taskID string) error {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	var maxReviewFixCycles int = 3 // default
	if o.projects != nil {
		if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.MaxReviewFixCycles > 0 {
			maxReviewFixCycles = p.MaxReviewFixCycles
		}
	}

	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return err
	}

	rejectionCount := 0
	for _, cp := range checkpoints {
		if cp.Step == "pr_rejection" {
			rejectionCount++
		}
	}

	if rejectionCount >= maxReviewFixCycles {
		_, _ = o.updateTaskStatus(ctx, taskID, models.TaskStatusFailed)
		o.log(ctx, taskID, nil, "error", fmt.Sprintf("review loop limit exceeded (limit: %d). task marked as failed.", maxReviewFixCycles))
		return fmt.Errorf("review loop limit of %d exceeded. task has failed", maxReviewFixCycles)
	}
	return nil
}

// PauseJob pauses the running workflow job for a task.
func (o *Orchestrator) PauseJob(ctx context.Context, taskID string) error {
	job, err := o.workflows.LatestByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	if job.Status != models.WorkflowJobStatusRunning {
		return fmt.Errorf("job is not running (status: %s)", job.Status)
	}

	_, err = o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusPaused})
	if err != nil {
		return err
	}

	if cancelVal, ok := o.jobCancels.Load(job.ID); ok {
		if cancel, ok := cancelVal.(context.CancelFunc); ok {
			cancel()
		}
	}

	o.wake()
	return nil
}

// CancelJob cancels/closes the running workflow job for a task.
func (o *Orchestrator) CancelJob(ctx context.Context, taskID string) error {
	job, err := o.workflows.LatestByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	if job.Status != models.WorkflowJobStatusRunning && job.Status != models.WorkflowJobStatusQueued && job.Status != models.WorkflowJobStatusPaused {
		return fmt.Errorf("job is not running, queued, or paused (status: %s)", job.Status)
	}

	_, err = o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusFailed, "last_error": "cancelled by user"})
	if err != nil {
		return err
	}

	if _, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusFailed); err != nil {
		return err
	}

	if cancelVal, ok := o.jobCancels.Load(job.ID); ok {
		if cancel, ok := cancelVal.(context.CancelFunc); ok {
			cancel()
		}
	}

	o.releaseWorkspaceLock(taskID)
	o.wake()
	return nil
}



