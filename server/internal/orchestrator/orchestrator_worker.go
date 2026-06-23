package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

func (o *Orchestrator) StartWorker(ctx context.Context, interval time.Duration, concurrency int) {
	if interval <= 0 {
		interval = 20 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	
	// Recover stuck agents and jobs from previous crashes
	if o.agents != nil {
		if mgr, ok := o.agents.(*AgentManager); ok && mgr.repo != nil {
			_ = mgr.repo.ResetAllStatuses(ctx)
		}
	}
	_ = o.workflows.ResetStuckJobs(ctx)

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

	if err := o.ensureWorkspaceCloned(ctx, task, agent); err != nil {
		o.fail(ctx, job, fmt.Errorf("workspace clone failed: %w", err))
		return
	}

	if task.Status == models.TaskStatusTodo || task.Status == models.TaskStatusFailed || task.Status == "" {
		if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusContextLoading); err != nil {
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

	runners := o.stepRunners(task, agent, job.ID)
	var def workflow.Definition
	switch task.Complexity {
	case models.TaskComplexityEasy:
		def = workflow.EasyWorkflow(runners)
	case models.TaskComplexityHard:
		def = workflow.HardWorkflow(runners)
	default:
		def = workflow.MediumWorkflow(runners)
	}

	// Load existing checkpoints to support resume-from-last-success.
	if checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID); cpErr == nil && len(checkpoints) > 0 {
		completed := make(map[string]map[string]any)
		for _, cp := range checkpoints {
			var state map[string]any
			if json.Unmarshal(cp.State, &state) == nil {
				if status, _ := state["status"].(string); status == workflow.StepStatusSuccess {
					if job.Step == workflow.StepReview && (cp.Step == workflow.StepReview || cp.Step == workflow.StepFix) {
						continue
					}
					output, _ := state["output"].(map[string]any)
					if output == nil {
						output = map[string]any{}
					}
					completed[cp.Step] = output
				}
			}
		}
		if len(completed) > 0 {
			engine.CompletedSteps = completed
			o.log(ctx, task.ID, &job.ID, "info", fmt.Sprintf("resuming workflow with %d completed steps from checkpoint", len(completed)))
		}
	}
	maxRetries := 1
	if o.projects != nil {
		if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.MaxRetries > 0 {
			maxRetries = p.MaxRetries
		}
	}

	var result workflow.Result
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err = engine.Run(ctx, def, map[string]any{"task_id": task.ID, "agent_id": agent.ID, "job_id": job.ID})
		if errors.Is(err, workflow.ErrReviewFixLoop) {
			o.log(ctx, task.ID, &job.ID, "info", "Review findings detected. Looping back to review step.")
			_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
				"status": models.WorkflowJobStatusQueued,
				"step":   workflow.StepReview,
			})
			return
		}
		if err == nil || errors.Is(err, workflow.ErrPaused) {
			break
		}
		if attempt < maxRetries {
			o.log(ctx, task.ID, &job.ID, "warn", fmt.Sprintf("Workflow failed: %v. Retrying attempt %d of %d in 2s...", err, attempt+1, maxRetries))
			time.Sleep(2 * time.Second)
		}
	}

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
	defer o.cleanupWorkspaceAfterFinalState(cleanupCtx, task.ID)
	if _, err := o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusDone, "step": models.WorkflowStepDone}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	_ = o.checkpoint(cleanupCtx, task.ID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone, "steps": result.Status})
	o.log(cleanupCtx, task.ID, &job.ID, "info", "workflow completed and is waiting for human PR approval")
}

func (o *Orchestrator) fail(ctx context.Context, job *models.WorkflowJob, err error) {
	cleanupCtx := context.WithoutCancel(ctx)
	defer o.cleanupWorkspaceAfterFinalState(cleanupCtx, job.TaskID)
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
