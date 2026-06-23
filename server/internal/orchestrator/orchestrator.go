package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AgentAssigner interface {
	Assign(ctx context.Context, task *models.Task) (*models.Agent, error)
	AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error)
	MarkRunning(ctx context.Context, agentID string) error
	Release(ctx context.Context, agentID string) error
}

type PromptBuilder interface {
	Assemble(ctx context.Context, task models.Task) ([]llm.Message, []ToolDefinition, error)
	AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []ToolDefinition, error)
}

type GitOpsClient interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	// CloneForTask clones a repository resolving credentials from the linked GitAccount.
	// Use this instead of CloneRepo inside the orchestrator so the git account token
	// is never stored in the task/repo model and is always resolved at clone time.
	CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error)
	CreateBranch(ctx context.Context, localPath, repoURL, branchName string) error
	CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error
	CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
	MergePullRequest(ctx context.Context, repoURL, prURL string) error
}

type MemoryRecorder interface {
	SessionStart(ctx context.Context, agentID string, task *models.Task) ([]models.EpisodicMemory, error)
	PostStepRecord(ctx context.Context, agentID string, task *models.Task, sessionID, stepID, status string, output map[string]any)
	SessionEnd(ctx context.Context, agentID string, task *models.Task, sessionID, finalStatus string)
}

type LearningRecorder interface {
	ComputeConfidence(ctx context.Context, agentID, complexity string) float64
	EvaluateOutcome(ctx context.Context, task *models.Task, job *models.WorkflowJob)
	DetectPatterns(ctx context.Context, agentID string)
	SuggestRuleFromErrors(ctx context.Context, agentID string)
	SuggestPromptPatch(ctx context.Context, task *models.Task, job *models.WorkflowJob)
}

type ArtifactRepository interface {
	Create(ctx context.Context, artifact *models.WorkflowArtifact) error
	ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error)
	ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error)
}

type RepositoryRepository interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error)
}

type ProjectRepository interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

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
	workspaceRoot string
	retention     WorkspaceRetention
	wg            sync.WaitGroup
	lockCancels   sync.Map
	lockConns     sync.Map
}

type WorkspaceRetention struct {
	Retention time.Duration
	Interval  time.Duration
}

func defaultWorkspaceRetention() WorkspaceRetention {
	return WorkspaceRetention{Retention: 72 * time.Hour, Interval: time.Hour}
}

type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
}

type WorkflowRepository interface {
	Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	ClaimNext(ctx context.Context) (*models.WorkflowJob, error)
	LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	UpdateJob(ctx context.Context, jobID string, updates map[string]any) (*models.WorkflowJob, error)
	CreateCheckpoint(ctx context.Context, checkpoint models.WorkflowCheckpoint) error
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
	DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error
	CreateLog(ctx context.Context, log models.TaskLog) error
	ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error)
	ResetStuckJobs(ctx context.Context) error
	AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error)
	ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error
}

func NewOrchestrator(taskRepo TaskRepository, workflowRepo WorkflowRepository, agentManager AgentAssigner, runtime sandbox.Runtime) *Orchestrator {
	return &Orchestrator{tasks: taskRepo, workflows: workflowRepo, agents: agentManager, runtime: runtime, retention: defaultWorkspaceRetention()}
}

func NewOrchestratorWithPrompt(taskRepo TaskRepository, workflowRepo WorkflowRepository, agentManager AgentAssigner, runtime sandbox.Runtime, prompts PromptBuilder) *Orchestrator {
	return &Orchestrator{tasks: taskRepo, workflows: workflowRepo, agents: agentManager, runtime: runtime, prompts: prompts, retention: defaultWorkspaceRetention()}
}

func (o *Orchestrator) SetMemoryHooks(hooks MemoryRecorder) {
	o.memHooks = hooks
}

func (o *Orchestrator) SetLearningEngine(engine LearningRecorder) {
	o.learnEngine = engine
}

func (o *Orchestrator) SetGitOpsClient(client GitOpsClient) {
	o.gitOps = client
}

func (o *Orchestrator) SetArtifactRepository(repo ArtifactRepository) {
	o.artifacts = repo
}

func (o *Orchestrator) SetRepositoryRepository(repo RepositoryRepository) {
	o.repositories = repo
}

func (o *Orchestrator) SetProjectRepository(repo ProjectRepository) {
	o.projects = repo
}

func (o *Orchestrator) SetWorkspaceRoot(rootPath string) {
	o.workspaceRoot = rootPath
}

func (o *Orchestrator) SetWorkspaceRetention(retention, interval time.Duration) {
	o.retention = WorkspaceRetention{Retention: retention, Interval: interval}
}

func (o *Orchestrator) SetLLMProvider(provider llm.Provider) {
	o.llm = provider
}

func (o *Orchestrator) ListArtifacts(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error) {
	if o.artifacts == nil {
		return nil, fmt.Errorf("artifact repository not configured")
	}
	return o.artifacts.ListByJobID(ctx, jobID)
}

func (o *Orchestrator) Execute(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	if _, err := o.tasks.GetByID(ctx, taskID); err != nil {
		return nil, err
	}

	// Check if there is a paused job to resume
	if job, err := o.workflows.LatestByTaskID(ctx, taskID); err == nil && job.Status == models.WorkflowJobStatusPaused {
		updated, err := o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusQueued})
		if err != nil {
			return nil, err
		}
		o.log(ctx, taskID, &job.ID, "info", "paused workflow job resumed")
		return updated, nil
	}

	job, err := o.workflows.Enqueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, &job.ID, "info", "workflow job queued")
	return job, nil
}

// RestartFromLastStep re-enqueues a failed task for execution.
// The worker will automatically load checkpoints and skip steps
// that already succeeded, resuming from the first incomplete step.
func (o *Orchestrator) RestartFromLastStep(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != models.TaskStatusFailed {
		return nil, fmt.Errorf("can only restart failed tasks (current status: %s)", task.Status)
	}

	// Transition task back to analyzing so the workflow can start.
	if _, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusAnalyzing); err != nil {
		return nil, fmt.Errorf("failed to transition task for restart: %w", err)
	}

	job, err := o.workflows.Enqueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, &job.ID, "info", "workflow restarted from last successful checkpoint")
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
		failed := models.TaskStatusFailed
		_, _ = o.tasks.Update(ctx, taskID, models.UpdateTaskInput{Status: &failed})
		o.log(ctx, taskID, nil, "error", fmt.Sprintf("review loop limit exceeded (limit: %d). task marked as failed.", maxReviewFixCycles))
		return fmt.Errorf("review loop limit of %d exceeded. task has failed", maxReviewFixCycles)
	}
	return nil
}
