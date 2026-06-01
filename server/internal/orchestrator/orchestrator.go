package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

type AgentAssigner interface {
	Assign(ctx context.Context, task *models.Task) (*models.Agent, error)
	MarkRunning(ctx context.Context, agentID string) error
	Release(ctx context.Context, agentID string) error
}

type PromptBuilder interface {
	Assemble(ctx context.Context, task models.Task) ([]llm.Message, []ToolDefinition, error)
	AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []ToolDefinition, error)
}

type GitOpsClient interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	CreateBranch(ctx context.Context, repoURL, branchName string) error
	CommitAndPush(ctx context.Context, repoURL, branchName, message string, files map[string]string) error
	CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
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
	workspaceRoot string
	wg            sync.WaitGroup
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
	CreateLog(ctx context.Context, log models.TaskLog) error
	ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error)
}

func NewOrchestrator(taskRepo TaskRepository, workflowRepo WorkflowRepository, agentManager AgentAssigner, runtime sandbox.Runtime) *Orchestrator {
	return &Orchestrator{tasks: taskRepo, workflows: workflowRepo, agents: agentManager, runtime: runtime}
}

func NewOrchestratorWithPrompt(taskRepo TaskRepository, workflowRepo WorkflowRepository, agentManager AgentAssigner, runtime sandbox.Runtime, prompts PromptBuilder) *Orchestrator {
	return &Orchestrator{tasks: taskRepo, workflows: workflowRepo, agents: agentManager, runtime: runtime, prompts: prompts}
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

func (o *Orchestrator) SetWorkspaceRoot(rootPath string) {
	o.workspaceRoot = rootPath
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

	job, err := o.workflows.Enqueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, &job.ID, "info", "workflow job queued")
	return job, nil
}

func (o *Orchestrator) StartWorker(ctx context.Context, interval time.Duration, concurrency int) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		claimed := false
		for {
			select {
			case sem <- struct{}{}:
			default:
				goto wait
			}

			job, err := o.workflows.ClaimNext(ctx)
			if err != nil {
				<-sem
				if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, repository.ErrNotFound) {
					break
				}
				observability.Error(ctx, "claim workflow job failed", "error", err)
				break
			}
			claimed = true
			o.wg.Add(1)
			go func(jobID string) {
				defer o.wg.Done()
				defer func() { <-sem }()
				o.run(ctx, jobID)
			}(job.ID)
		}

	wait:
		if claimed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (o *Orchestrator) Wait() {
	o.wg.Wait()
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
	if task.Status != models.TaskStatusHumanReview {
		return nil, fmt.Errorf("task is not waiting for human PR approval")
	}
	updated, err := o.updateTaskStatus(ctx, taskID, models.TaskStatusMerged)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, nil, "info", "human approved workflow for merge")
	return updated, nil
}

func (o *Orchestrator) updateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := workflow.ValidateTaskTransition(task.Status, newStatus); err != nil {
		return nil, fmt.Errorf("invalid task status transition from %q to %q: %w", task.Status, newStatus, err)
	}
	return o.tasks.Update(ctx, taskID, models.UpdateTaskInput{Status: &newStatus})
}

func (o *Orchestrator) run(ctx context.Context, jobID string) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.run")
	defer span.End()
	job, err := o.workflows.UpdateJob(ctx, jobID, map[string]any{"status": models.WorkflowJobStatusRunning})
	if err != nil {
		observability.Error(ctx, "workflow job start failed", "job_id", jobID, "error", err)
		return
	}
	ctx = observability.WithTaskID(ctx, job.TaskID)

	task, err := o.tasks.GetByID(ctx, job.TaskID)
	if err != nil {
		o.fail(ctx, job, err)
		return
	}

	if err := o.ensureWorkspaceCloned(ctx, task); err != nil {
		o.fail(ctx, job, fmt.Errorf("workspace clone failed: %w", err))
		return
	}

	if err := o.checkpoint(ctx, task.ID, &job.ID, models.WorkflowStepAssign, map[string]any{"status": workflow.StepStatusRunning}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	agent, err := o.agents.Assign(ctx, task)
	if err != nil {
		o.fail(ctx, job, err)
		return
	}
	if _, err := o.workflows.UpdateJob(ctx, job.ID, map[string]any{"agent_id": agent.ID, "step": models.WorkflowStepAssign}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	ctx = observability.WithAgentID(ctx, agent.ID)
	if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{AgentID: &agent.ID}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	o.log(ctx, task.ID, &job.ID, "info", fmt.Sprintf("assigned agent %s", agent.Name))

	if task.Status == models.TaskStatusTodo || task.Status == models.TaskStatusAssigned || task.Status == "" {
		if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusAnalyzing); err != nil {
			o.fail(ctx, job, err)
			return
		}
	}

	if err := o.agents.MarkRunning(ctx, agent.ID); err != nil {
		o.fail(ctx, job, err)
		return
	}
	defer func() {
		if err := o.agents.Release(context.WithoutCancel(ctx), agent.ID); err != nil {
			observability.Warn(ctx, "release agent failed", "error", err)
		}
	}()

	// Generate a unique session ID for this workflow run
	sessionID := NewSessionID()

	// Load relevant memories and inject into context
	if o.memHooks != nil {
		memories, err := o.memHooks.SessionStart(ctx, agent.ID, task)
		if err == nil && len(memories) > 0 {
			ctx = context.WithValue(ctx, memoriesCtxKey, memories)
		}
	}

	// Compute and record agent confidence score
	var confidence float64 = 0.5
	if o.learnEngine != nil {
		confidence = o.learnEngine.ComputeConfidence(ctx, agent.ID, task.Complexity)
	}
	_ = o.checkpoint(ctx, task.ID, &job.ID, "agent_confidence", MarshalConfidenceToCheckpoint(confidence))

	engine := &workflow.Engine{
		MaxParallel: 2,
		OnEvent: func(ctx context.Context, event workflow.Event) error {
			updates := map[string]any{"step": event.StepID}
			if event.Status == workflow.StepStatusPaused {
				updates["status"] = models.WorkflowJobStatusPaused
				updates["last_error"] = event.Error
			}
			if event.Status == workflow.StepStatusFailed {
				updates["last_error"] = event.Error
			}
			if _, err := o.workflows.UpdateJob(ctx, job.ID, updates); err != nil {
				return err
			}
			state := map[string]any{"status": event.Status}
			if event.Output != nil {
				state["output"] = event.Output
			}
			if event.Error != "" {
				state["error"] = event.Error
			}
			if err := o.checkpoint(ctx, task.ID, &job.ID, event.StepID, state); err != nil {
				return err
			}
			o.log(ctx, task.ID, &job.ID, "info", fmt.Sprintf("step %s %s", event.StepID, event.Status))

			// Record step observation memory
			if o.memHooks != nil {
				o.memHooks.PostStepRecord(ctx, agent.ID, task, sessionID, event.StepID, string(event.Status), event.Output)
			}
			return nil
		},
	}

	def := workflow.DefaultWorkflow(o.stepRunners(task, agent, job.ID))
	result, err := engine.Run(ctx, def, map[string]any{"task_id": task.ID, "agent_id": agent.ID, "job_id": job.ID})

	finalStatus := models.WorkflowJobStatusDone
	var finalErr string
	if err != nil {
		if errors.Is(err, workflow.ErrPaused) {
			finalStatus = models.WorkflowJobStatusPaused
			finalErr = err.Error()
		} else {
			finalStatus = models.WorkflowJobStatusFailed
			finalErr = err.Error()
		}
	}

	// Update job state locally for evaluation
	updatedJob, getErr := o.workflows.LatestByTaskID(ctx, task.ID)
	if getErr != nil || updatedJob == nil {
		updatedJob = job
	}
	updatedJob.Status = finalStatus
	updatedJob.LastError = finalErr

	// End memory session
	if o.memHooks != nil {
		o.memHooks.SessionEnd(ctx, agent.ID, task, sessionID, finalStatus)
	}

	// Post-task learning evaluation and improvements suggestions
	if o.learnEngine != nil && finalStatus != models.WorkflowJobStatusPaused {
		leCtx := context.WithoutCancel(ctx)
		leJob := updatedJob
		leTask := task
		go func() {
			le := o.learnEngine
			le.EvaluateOutcome(leCtx, leTask, leJob)
			if finalStatus == models.WorkflowJobStatusDone {
				le.DetectPatterns(leCtx, agent.ID)
				le.SuggestRuleFromErrors(leCtx, agent.ID)
			} else if finalStatus == models.WorkflowJobStatusFailed {
				le.SuggestPromptPatch(leCtx, leTask, leJob)
			}
		}()
	}

	if err != nil {
		if errors.Is(err, workflow.ErrPaused) {
			cleanupCtx := context.WithoutCancel(ctx)
			_, _ = o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusPaused, "last_error": err.Error()})
			o.log(cleanupCtx, task.ID, &job.ID, "info", err.Error())
			return
		}
		o.fail(ctx, job, err)
		return
	}
	cleanupCtx := context.WithoutCancel(ctx)
	if _, err := o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusDone, "step": models.WorkflowStepDone}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	_ = o.checkpoint(cleanupCtx, task.ID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone, "steps": result.Status})
	o.log(cleanupCtx, task.ID, &job.ID, "info", "workflow completed and is waiting for human PR approval")
}

func (o *Orchestrator) fail(ctx context.Context, job *models.WorkflowJob, err error) {
	cleanupCtx := context.WithoutCancel(ctx)
	failedStatus := models.TaskStatusFailed
	if _, updateErr := o.updateTaskStatus(cleanupCtx, job.TaskID, failedStatus); updateErr != nil {
		observability.Error(cleanupCtx, "mark task failed", "job_id", job.ID, "task_id", job.TaskID, "error", updateErr, "cause", err)
	}
	if _, updateErr := o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusFailed, "last_error": err.Error()}); updateErr != nil {
		observability.Error(cleanupCtx, "mark workflow failed", "job_id", job.ID, "error", updateErr, "cause", err)
	}
	o.log(cleanupCtx, job.TaskID, &job.ID, "error", err.Error())
}

func (o *Orchestrator) checkpoint(ctx context.Context, taskID string, jobID *string, step string, state map[string]any) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return o.workflows.CreateCheckpoint(ctx, models.WorkflowCheckpoint{TaskID: taskID, JobID: jobID, Step: step, State: raw})
}

func (o *Orchestrator) log(ctx context.Context, taskID string, jobID *string, level, message string) {
	if err := o.workflows.CreateLog(ctx, models.TaskLog{TaskID: taskID, JobID: jobID, Level: level, Message: message}); err != nil {
		slog.Warn("persist workflow log failed", observability.LogAttrs(ctx, "task_id", taskID, "job_id", jobID, "level", level, "error", err)...)
	}
	switch level {
	case "error":
		observability.Error(ctx, message, "job_id", jobID)
	case "warn":
		observability.Warn(ctx, message, "job_id", jobID)
	default:
		observability.Info(ctx, message, "job_id", jobID)
	}
}

func taskReadyForExecution(task *models.Task) bool {
	switch task.SpecStatus {
	case models.TaskSpecStatusApproved, models.TaskSpecStatusAutoApproved:
		return true
	default:
		return false
	}
}

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string) map[string]workflow.StepFunc {
	return map[string]workflow.StepFunc{
		workflow.StepAnalyze: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if o.prompts != nil {
				messages, tools, err := o.prompts.AssembleForAgent(ctx, *task, agent, nil)
				if err != nil {
					return nil, err
				}
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
			}
			if taskReadyForExecution(task) {
				return map[string]any{"complexity": task.Complexity, "spec_status": task.SpecStatus}, nil
			}
			analysis := deriveWorkflowAnalysis(task)
			raw, err := json.Marshal(analysis)
			if err != nil {
				return nil, err
			}
			specStatus := models.TaskSpecStatusPendingReview
			status := models.TaskStatusSpecReview
			if len(analysis.ClarificationQuestions) > 0 {
				specStatus = models.TaskSpecStatusChangesRequested
			} else if analysis.Complexity == models.TaskComplexityEasy {
				specStatus = models.TaskSpecStatusAutoApproved
				status = models.TaskStatusCoding
			} else {
				specStatus = models.TaskSpecStatusAutoApproved
				status = models.TaskStatusPlanning
			}
			if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{
				Complexity: &analysis.Complexity,
				Analysis:   raw,
				SpecStatus: &specStatus,
			}); err != nil {
				return nil, err
			}
			if _, err := o.updateTaskStatus(ctx, task.ID, status); err != nil {
				return nil, err
			}
			task.Complexity = analysis.Complexity
			task.SpecStatus = specStatus
			task.Analysis = raw
			if specStatus == models.TaskSpecStatusPendingReview || specStatus == models.TaskSpecStatusChangesRequested {
				return nil, workflow.PauseError{Step: workflow.StepAnalyze, Reason: "workflow paused for human spec review"}
			}
			return map[string]any{"complexity": analysis.Complexity, "spec_status": specStatus}, nil
		},
		workflow.StepPlan: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			var out map[string]any
			var err error
			if o.llm != nil {
				out, err = o.runLLMStep(ctx, task, agent, jobID, workflow.StepPlan, "Create a concise JSON execution plan with subtasks, risks, and test strategy.")
			} else {
				plan := []any{
					map[string]any{"id": "backend", "role": models.AgentRoleBackend, "description": "Implement server-side changes and data contracts."},
					map[string]any{"id": "frontend", "role": models.AgentRoleFrontend, "description": "Implement user-facing workflow updates when applicable."},
				}
				out, err = map[string]any{"subtasks": plan}, nil
			}
			if err != nil {
				return nil, err
			}
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusCoding); err != nil {
				return nil, err
			}
			return out, nil
		},
		workflow.StepCodeBackend: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepCodeBackend, "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, agent, workflow.StepCodeBackend, patch); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepCodeBackend); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "diff", diffText)
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepCodeFrontend: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepCodeFrontend, "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, agent, workflow.StepCodeFrontend, patch); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepCodeFrontend); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "diff", diffText)
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepMerge: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			diffText, err := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepMerge)
			if err != nil {
				return nil, fmt.Errorf("merge check failed: %w", err)
			}
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
				return nil, err
			}
			return map[string]any{
				"status":    "changes_reconciled",
				"info":      "local changes reconciled",
				"diff_size": len(diffText),
			}, nil
		},
		workflow.StepReview: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if o.llm != nil {
				diffText, _ := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepReview)
				instruction := "Review the proposed changes. Here is the current workspace diff:\n\n" + diffText + "\n\nReturn JSON findings with severity, file, line, and recommendation."
				out, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepReview, instruction)
				if err != nil {
					return nil, err
				}
				hasFindings := true
				if parsed, ok := out["parsed"].(map[string]any); ok {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepReview, "review_findings", parsed)
					if findings, exists := parsed["findings"]; exists {
						if slice, ok := findings.([]any); ok && len(slice) == 0 {
							hasFindings = false
						}
					}
				}
				nextStatus := models.TaskStatusFixing
				if !hasFindings {
					nextStatus = models.TaskStatusTesting
				}
				if _, err := o.updateTaskStatus(ctx, task.ID, nextStatus); err != nil {
					return nil, err
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepFix: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
				if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
					if findings, exists := parsed["findings"]; exists {
						if slice, ok := findings.([]any); ok && len(slice) == 0 {
							return map[string]any{
								"status": "skipped",
								"info":   "no review findings, skipped fix step",
							}, nil
						}
					}
				}
			}
			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepFix, "Fix review findings. Return JSON with fixes_applied, files_changed, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, agent, workflow.StepFix, patch); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepFix); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "diff", diffText)
				}
				if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
					return nil, err
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepTest: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting); err != nil {
				return nil, err
			}
			out, err := o.runSandboxStep(ctx, task, agent, workflow.StepTest, `if [ -f go.mod ]; then go test ./...; elif [ -f package.json ]; then npm test; else echo "no supported test runner found"; fi`)
			if err != nil {
				return nil, err
			}
			out["exit_code"] = 0
			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepTest, "test_output", out)
			return out, nil
		},
		workflow.StepPR: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if o.gitOps == nil {
				return nil, fmt.Errorf("gitops client is not configured")
			}
			repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("list project repositories: %w", err)
			}
			if len(repos) == 0 {
				return nil, fmt.Errorf("no repository linked to project %s", task.ProjectID)
			}
			repo := repos[0]
			branchName := fmt.Sprintf("autocode/task-%s", task.ID)

			if err := o.gitOps.CreateBranch(ctx, repo.URL, branchName); err != nil {
				return nil, fmt.Errorf("create branch %s: %w", branchName, err)
			}

			commitMsg := fmt.Sprintf("AutoCodeOS: implement task %s\n\nTitle: %s", task.ID, task.Title)
			if err := o.gitOps.CommitAndPush(ctx, repo.URL, branchName, commitMsg, nil); err != nil {
				return nil, fmt.Errorf("commit and push to branch %s: %w", branchName, err)
			}

			prTitle := fmt.Sprintf("AutoCodeOS: %s", task.Title)
			prBody := fmt.Sprintf("This Pull Request was automatically generated by AutoCodeOS agent for task %s.\n\nDescription:\n%s", task.ID, task.Description)
			prURL, err := o.gitOps.CreatePullRequest(ctx, repo.URL, branchName, prTitle, prBody)
			if err != nil {
				return nil, fmt.Errorf("create PR: %w", err)
			}

			status := models.TaskStatusHumanReview
			if _, err := o.updateTaskStatus(ctx, task.ID, status); err != nil {
				return nil, err
			}

			return map[string]any{
				"status": "pr_ready_for_human_approval",
				"branch": branchName,
				"pr_url": prURL,
			}, nil
		},
	}
}

func (o *Orchestrator) runLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	if o.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	var messages []llm.Message
	var err error
	if o.prompts != nil {
		messages, _, err = o.prompts.AssembleForAgent(ctx, *task, agent, nil)
		if err != nil {
			return nil, err
		}
	} else {
		messages = []llm.Message{{Role: "user", Content: task.Title + "\n\n" + task.Description}}
	}
	messages = append(messages, llm.Message{Role: "user", Content: "Workflow step: " + stepID + "\n\n" + instruction})

	// Save prompt artifact
	_ = o.saveArtifact(ctx, jobID, task.ID, stepID, "prompt", messages)

	ctx = llm.WithRouteOptions(ctx, llm.RouteOptions{
		Complexity: task.Complexity,
		ProjectID:  task.ProjectID,
		AgentID:    agent.ID,
		TaskID:     task.ID,
	})
	resp, err := o.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: llm response from %s", stepID, resp.Model))
	var parsed map[string]any
	if parsedJSON, err := parseJSONMarkdown(resp.Content); err == nil {
		parsed = parsedJSON
	} else {
		parsed = map[string]any{"raw_content": resp.Content}
	}

	// Save llm_response artifact
	_ = o.saveArtifact(ctx, jobID, task.ID, stepID, "llm_response", parsed)

	return map[string]any{
		"status":        "llm_completed",
		"model":         resp.Model,
		"content":       resp.Content,
		"parsed":        parsed,
		"prompt_tokens": resp.PromptTokens,
		"output_tokens": resp.OutputTokens,
	}, nil
}

func (o *Orchestrator) runSandboxStep(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func deriveWorkflowAnalysis(task *models.Task) models.TaskAnalysis {
	text := strings.ToLower(task.Title + " " + task.Description)
	complexity := task.Complexity
	if complexity == "" {
		complexity = models.TaskComplexityEasy
	}
	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}
	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = models.TaskComplexityHard
			break
		}
	}
	if complexity != models.TaskComplexityHard {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = models.TaskComplexityMedium
				break
			}
		}
	}
	questions := []string{}
	if len(strings.TrimSpace(task.Description)) < 30 {
		questions = append(questions, "Please provide more implementation context, affected module names, and expected behavior.")
	}
	return models.TaskAnalysis{
		Complexity:    complexity,
		Scope:         "Generated by the Phase 3b workflow analyze step.",
		AffectedFiles: []string{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPlan: []string{
			"Assemble prompt with role, rules, and retrieved context.",
			"Decompose work into typed subtasks.",
			"Run backend and frontend coding tracks in parallel sandboxes.",
			"Merge, review, fix, test, and prepare PR approval checkpoint.",
		},
		ClarificationQuestions: questions,
	}
}

func (o *Orchestrator) ensureWorkspaceCloned(ctx context.Context, task *models.Task) error {
	if o.repositories == nil {
		return fmt.Errorf("repositories lookup not configured")
	}
	if o.gitOps == nil {
		return fmt.Errorf("gitops client not configured")
	}
	repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
	if err != nil {
		return fmt.Errorf("list project repositories: %w", err)
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repository linked to project %s", task.ProjectID)
	}
	repo := repos[0]

	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	gitDir := filepath.Join(localPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// Already cloned
		return nil
	}

	// Clean target directory just in case
	os.RemoveAll(localPath)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create workspace parent dir: %w", err)
	}

	_, err = o.gitOps.CloneRepo(ctx, repo.URL, repo.Token, repo.Branch, localPath)
	if err != nil {
		return fmt.Errorf("clone repo: %w", err)
	}

	return nil
}

func parseJSONMarkdown(content string) (map[string]any, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if strings.HasSuffix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(trimmed), &res); err != nil {
		start := strings.Index(trimmed, "{")
		end := strings.LastIndex(trimmed, "}")
		if start != -1 && end != -1 && end > start {
			trimmed = trimmed[start : end+1]
			if err := json.Unmarshal([]byte(trimmed), &res); err == nil {
				return res, nil
			}
		}
		return nil, err
	}
	return res, nil
}

func (o *Orchestrator) applyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string) error {
	if patchText == "" {
		return nil
	}
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	fullPath := filepath.Join(localPath, "patch.diff")
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(fullPath, []byte(patchText), 0o644); err != nil {
		return err
	}
	_, err := o.runSandboxStep(ctx, task, agent, stepID+"_apply_patch", "git apply patch.diff")
	if err != nil {
		return fmt.Errorf("git apply patch: %w", err)
	}
	_, _ = o.runSandboxStep(ctx, task, agent, stepID+"_clean_patch", "rm patch.diff")
	return nil
}

func (o *Orchestrator) captureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string) (string, error) {
	out, err := o.runSandboxStep(ctx, task, agent, stepID+"_git_diff", "git diff")
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	diffText, _ := out["stdout"].(string)
	return diffText, nil
}

func (o *Orchestrator) saveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	if o.artifacts == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	artifact := &models.WorkflowArtifact{
		JobID:   jobID,
		TaskID:  taskID,
		Step:    step,
		Type:    artType,
		Payload: raw,
	}
	return o.artifacts.Create(ctx, artifact)
}

func extractPatch(parsed map[string]any) string {
	if parsed == nil {
		return ""
	}
	if p, ok := parsed["patch"].(string); ok && p != "" {
		return p
	}
	if p, ok := parsed["patch_text"].(string); ok && p != "" {
		return p
	}
	if p, ok := parsed["diff"].(string); ok && p != "" {
		return p
	}
	return ""
}
