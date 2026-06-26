package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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
	tasks         TaskRepository
	workflows     WorkflowRepository
	agents        AgentAssigner
	runtime       sandbox.Runtime
	prompts       PromptBuilder
	llm           llm.Provider
	memHooks      MemoryRecorder
	learnEngine   LearningRecorder
	gitOps        GitOpsClient
	artifacts     ArtifactRepository
	repositories  RepositoryRepository
	projects      ProjectRepository
	sandboxGit    gitops.SandboxGitClient
	workspaceRoot string
	retention     WorkspaceRetention
	wg            sync.WaitGroup
	lockCancels   sync.Map
	lockConns     sync.Map
	wkspace       *wkspace.Manager
	checkpoints   *checkpoint.Store
	repoutil      *repoutil.Manager
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

func WithWorkspaceRetention(retention, interval time.Duration) Option {
	return func(o *Orchestrator) {
		o.retention = WorkspaceRetention{Retention: retention, Interval: interval}
	}
}

func WithLLMProvider(provider llm.Provider) Option {
	return func(o *Orchestrator) {
		o.llm = provider
	}
}

func WithPrompts(prompts PromptBuilder) Option {
	return func(o *Orchestrator) {
		o.prompts = prompts
	}
}

func New(taskRepo TaskRepository, workflowRepo WorkflowRepository, agentManager AgentAssigner, runtime sandbox.Runtime, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		tasks:     taskRepo,
		workflows: workflowRepo,
		agents:    agentManager,
		runtime:   runtime,
		retention: defaultWorkspaceRetention(),
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
	if task.Status != models.TaskStatusFailed {
		return nil, fmt.Errorf("can only retry failed tasks (current status: %s)", task.Status)
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
	return o.workflows.DeleteCheckpoints(ctx, taskID, []string{"review", "fix", "test", "pr"})
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
		if err == nil {
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
	}

	updated, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusMerged)
	if err != nil {
		return nil, err
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

func (o *Orchestrator) initWkspace() {
	if o.wkspace == nil {
		o.wkspace = wkspace.NewManager(
			o.tasks,
			o.workflows,
			o.repositories,
			o.gitOps,
			o.artifacts,
			o.workspaceRoot,
			wkspace.WorkspaceRetention{
				Retention: o.retention.Retention,
				Interval:  o.retention.Interval,
			},
			o.log,
			func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, patchText string, worktreeSuffix string) error {
				o.initRepoutil()
				return o.repoutil.ApplyPatch(ctx, task, agent, stepName, patchText, worktreeSuffix)
			},
		)
	} else {
		o.wkspace.Tasks = o.tasks
		o.wkspace.Workflows = o.workflows
		o.wkspace.Repositories = o.repositories
		o.wkspace.GitOps = o.gitOps
		o.wkspace.Artifacts = o.artifacts
		o.wkspace.WorkspaceRoot = o.workspaceRoot
		o.wkspace.Retention = wkspace.WorkspaceRetention{
			Retention: o.retention.Retention,
			Interval:  o.retention.Interval,
		}
	}
}

func (o *Orchestrator) StartWorkspacePruner(ctx context.Context) {
	o.initWkspace()
	o.wkspace.StartWorkspacePruner(ctx)
}

func (o *Orchestrator) StartLogPruner(ctx context.Context, retentionDays int, fileRoot string) {
	o.initWkspace()
	o.wkspace.StartLogPruner(ctx, retentionDays, fileRoot)
}

func (o *Orchestrator) ensureWorkspaceCloned(ctx context.Context, task *models.Task, agent *models.Agent, jobID string) error {
	o.initWkspace()
	return o.wkspace.EnsureWorkspaceCloned(ctx, task, agent, jobID)
}

func (o *Orchestrator) cleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	o.initWkspace()
	o.wkspace.CleanupWorkspaceAfterFinalState(ctx, taskID)
}

func (o *Orchestrator) releaseWorkspaceLock(taskID string) {
	o.initWkspace()
	o.wkspace.ReleaseWorkspaceLock(taskID)
}

func (o *Orchestrator) RemoveWorkspace(taskID string) error {
	o.initWkspace()
	return o.wkspace.RemoveWorkspace(taskID)
}


func (o *Orchestrator) initCheckpoints() {
	if o.checkpoints == nil {
		o.checkpoints = checkpoint.NewStore(
			o.workflows,
			o.artifacts,
			o.log,
		)
	} else {
		o.checkpoints.Workflows = o.workflows
		o.checkpoints.Artifacts = o.artifacts
	}
}

func (o *Orchestrator) initRepoutil() {
	o.initWkspace()
	if o.repoutil == nil {
		var listRepos func(ctx context.Context, projectID string) ([]models.Repository, error)
		if o.repositories != nil {
			listRepos = o.repositories.ListByProjectID
		}
		var getChangedFiles func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
		var getDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error)
		var getWorkspaceDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, worktreeSuffix string) (string, error)
		var getPRDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error)
		if o.sandboxGit != nil {
			getChangedFiles = o.sandboxGit.GetChangedFiles
			getDiff = o.sandboxGit.GetDiff
			getWorkspaceDiff = o.sandboxGit.GetWorkspaceDiff
			getPRDiff = o.sandboxGit.GetPRDiff
		}
		o.repoutil = repoutil.NewManager(
			o.workspaceRoot,
			listRepos,
			o.wkspace.GetTaskWorkspace,
			o.wkspace.LoadTaskWorkspace,
			o.wkspace.SaveTaskWorkspaceMetadata,
			o.wkspace.FindRepoWorkspaceByPath,
			o.containerPathForHostPath,
			o.runSandboxStep,
			o.runSandboxStepInWorktree,
			getChangedFiles,
			getDiff,
			getWorkspaceDiff,
			getPRDiff,
			o.log,
		)
	} else {
		o.repoutil.WorkspaceRoot = o.workspaceRoot
		if o.repositories != nil {
			o.repoutil.ListRepositories = o.repositories.ListByProjectID
		}
		if o.sandboxGit != nil {
			o.repoutil.SandboxGitGetChangedFiles = o.sandboxGit.GetChangedFiles
			o.repoutil.SandboxGitGetDiff = o.sandboxGit.GetDiff
			o.repoutil.SandboxGitGetWorkspaceDiff = o.sandboxGit.GetWorkspaceDiff
			o.repoutil.SandboxGitGetPRDiff = o.sandboxGit.GetPRDiff
		}
	}
}

