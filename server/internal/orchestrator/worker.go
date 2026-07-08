package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/learning"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"go.opentelemetry.io/otel"
)


func (o *Orchestrator) run(ctx context.Context, jobID string) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.run")
	defer span.End()

	var taskID string
	var job *models.WorkflowJob
	var assignedAgentID string

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			err := fmt.Errorf("panic recovered: %v\nStack trace:\n%s", r, stack)
			observability.Error(ctx, "panic in workflow execution", "job_id", jobID, "error", err)

			cleanupCtx := context.WithoutCancel(ctx)
			if taskID != "" {
				o.cleanupWorkspaceAfterFinalState(cleanupCtx, taskID)
				failedStatus := models.TaskStatusFailed
				_, _ = o.updateTaskStatus(cleanupCtx, taskID, failedStatus)
				o.log(cleanupCtx, taskID, &jobID, "error", err.Error())
			}
			if job != nil {
				_, _ = o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusFailed, "last_error": err.Error()})
			} else {
				_, _ = o.workflows.UpdateJob(cleanupCtx, jobID, map[string]any{"status": models.WorkflowJobStatusFailed, "last_error": err.Error()})
			}
		}

		// Guarantee agent release on ALL exit paths (including early returns and panics).
		if assignedAgentID != "" {
			if err := o.agents.Release(context.WithoutCancel(context.Background()), assignedAgentID); err != nil {
				observability.Warn(context.Background(), "release agent failed", "error", err)
			}
		}

		// Guarantee workspace lock release on ALL exit paths.
		if taskID != "" {
			o.releaseWorkspaceLock(taskID)
		}
	}()

	var err error
	job, err = o.workflows.UpdateJob(ctx, jobID, map[string]any{"status": models.WorkflowJobStatusRunning})
	if err != nil {
		observability.Error(ctx, "workflow job start failed", "job_id", jobID, "error", err)
		return
	}
	taskID = job.TaskID
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
	assignedAgentID = agent.ID
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

	if err := o.ensureWorkspaceCloned(ctx, task, agent, job.ID); err != nil {
		o.fail(ctx, job, fmt.Errorf("workspace clone failed: %w", err))
		return
	}
	// Lock release is handled in the top-level defer

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

	// Generate a unique session ID for this workflow run
	sessionID := learning.NewSessionID()

	// Load relevant memories and inject into context
	if o.memHooks != nil {
		memories, err := o.memHooks.SessionStart(ctx, agent.ID, task)
		if err == nil && len(memories) > 0 {
			ctx = context.WithValue(ctx, prompts.MemoriesCtxKey, memories)
		}
	}

	// Compute and record agent confidence score
	var confidence float64 = 0.5
	if o.learnEngine != nil {
		confidence = o.learnEngine.ComputeConfidence(ctx, agent.ID, task.Complexity)
	}
	_ = o.checkpoint(ctx, task.ID, &job.ID, "agent_confidence", learning.MarshalConfidenceToCheckpoint(confidence))

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
			
			// Attach current SpecHash to the checkpoint state to ensure immutable contract resuming
			if len(task.Analysis) > 0 {
				var analysis models.TaskAnalysis
				if json.Unmarshal(task.Analysis, &analysis) == nil && analysis.SpecHash != "" {
					state["spec_hash"] = analysis.SpecHash
				}
			}
			if err := o.checkpoint(ctx, task.ID, &job.ID, event.StepID, state); err != nil {
				return err
			}
			o.log(ctx, task.ID, &job.ID, "info", fmt.Sprintf("step %s %s", event.StepID, event.Status))

			// Write to workflow_timeline.jsonl
			if ws := o.wkspace.GetTaskWorkspace(task); ws != nil {
				timelineFile := filepath.Join(ws.Root, "artifacts", "workflow_timeline.jsonl")
				timelineEvent := map[string]any{
					"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
					"step":      event.StepID,
					"status":    event.Status,
				}
				if event.Error != "" {
					timelineEvent["error"] = event.Error
				}
				if b, err := json.Marshal(timelineEvent); err == nil {
					f, err := os.OpenFile(timelineFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						f.Write(append(b, '\n'))
						f.Close()
					}
				}
			}

			// Record step observation memory
			if o.memHooks != nil {
				o.memHooks.PostStepRecord(ctx, agent.ID, task, sessionID, event.StepID, string(event.Status), event.Output)
			}
			
			if event.StepID == workflow.StepPlan && event.Status == workflow.StepStatusSuccess {
				hasSubtasks := false
				if subtasks, ok := event.Output["subtasks"].(map[string][]string); ok {
					if len(subtasks["backend"]) > 0 || len(subtasks["frontend"]) > 0 {
						hasSubtasks = true
					}
				} else if subtasksAny, ok := event.Output["subtasks"].(map[string]any); ok {
					if be, ok := subtasksAny["backend"].([]any); ok && len(be) > 0 {
						hasSubtasks = true
					}
					if fe, ok := subtasksAny["frontend"].([]any); ok && len(fe) > 0 {
						hasSubtasks = true
					}
				}
				if hasSubtasks {
					return workflow.ErrGraphChanged
				}
			}
			
			return nil
		},
	}

	var subtasks map[string][]string
	if len(task.Analysis) > 0 {
		var analysis models.TaskAnalysis
		if json.Unmarshal(task.Analysis, &analysis) == nil {
			subtasks = workflow.ParseTasksMD(analysis.TasksMD)
		}
	}

	runners := o.stepRunners(task, agent, job.ID, job.Step)
	var def workflow.Definition
	if len(task.Analysis) > 0 {
		var analysis models.TaskAnalysis
		if json.Unmarshal(task.Analysis, &analysis) == nil && len(analysis.ExecutionUnits) > 0 {
			def = workflow.DynamicDAGWorkflow(runners, analysis.ExecutionUnits)
		}
	}

	if def.Name == "" {
		switch task.Complexity {
		case models.TaskComplexityEasy:
			def = workflow.EasyWorkflow(runners)
		case models.TaskComplexityHard:
			def = workflow.HardWorkflow(runners, subtasks)
		default:
			def = workflow.MediumWorkflow(runners, subtasks)
		}
	}

	// Load existing checkpoints to support resume-from-last-success.
	var resumedStepID string
	if checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID); cpErr == nil && len(checkpoints) > 0 {
		var currentSpecHash string
		if len(task.Analysis) > 0 {
			var analysis models.TaskAnalysis
			if json.Unmarshal(task.Analysis, &analysis) == nil {
				currentSpecHash = analysis.SpecHash
			}
		}

		completed := make(map[string]map[string]any)
		for _, cp := range checkpoints {
			var state map[string]any
			if json.Unmarshal(cp.State, &state) == nil {
				// Validate SpecHash: if the contract changed, throw away the checkpoint
				if cpHash, ok := state["spec_hash"].(string); ok && cpHash != "" && currentSpecHash != "" && cpHash != currentSpecHash {
					o.log(ctx, task.ID, &job.ID, "warn", fmt.Sprintf("SpecHash mismatch at step %s. Discarding checkpoint to force re-execution.", cp.Step))
					continue
				}

				if status, _ := state["status"].(string); status == workflow.StepStatusSuccess {
					if job.Step == workflow.StepReview && (cp.Step == workflow.StepReview || cp.Step == workflow.StepFix) {
						continue
					}
					output, _ := state["output"].(map[string]any)
					if output == nil {
						output = map[string]any{}
					}
					completed[cp.Step] = output
				} else if status == workflow.StepStatusPaused || status == workflow.StepStatusWaitingApproval {
					resumedStepID = cp.Step
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
		attemptCtx := context.WithValue(ctx, "workflow_attempt", attempt)

		// Reload checkpoints to catch successes from previous attempts
		if checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID); cpErr == nil && len(checkpoints) > 0 {
			var currentSpecHash string
			if len(task.Analysis) > 0 {
				var analysis models.TaskAnalysis
				if json.Unmarshal(task.Analysis, &analysis) == nil {
					currentSpecHash = analysis.SpecHash
				}
			}

			completed := make(map[string]map[string]any)
			for _, cp := range checkpoints {
				var state map[string]any
				if json.Unmarshal(cp.State, &state) == nil {
					if cpHash, ok := state["spec_hash"].(string); ok && cpHash != "" && currentSpecHash != "" && cpHash != currentSpecHash {
						continue
					}

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
			engine.CompletedSteps = completed
		}

		if resumedStepID != "" {
			result, err = engine.Resume(attemptCtx, def, map[string]any{"task_id": task.ID, "agent_id": agent.ID, "job_id": job.ID}, resumedStepID)
		} else {
			result, err = engine.Run(attemptCtx, def, map[string]any{"task_id": task.ID, "agent_id": agent.ID, "job_id": job.ID})
		}
		if errors.Is(err, workflow.ErrReviewFixLoop) {
			o.log(ctx, task.ID, &job.ID, "info", "Review findings detected. Looping back to review step.")
			_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
				"status":     models.WorkflowJobStatusQueued,
				"step":       workflow.StepReview,
				"last_error": "",
			})
			return
		}
		if errors.Is(err, workflow.ErrGraphChanged) {
			o.log(ctx, task.ID, &job.ID, "info", "Workflow graph dynamically changed due to complexity update. Re-queueing job.")
			_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
				"status":     models.WorkflowJobStatusQueued,
				"last_error": "",
			})
			return
		}
		if err == nil || errors.Is(err, workflow.ErrPaused) || errors.Is(err, workflow.ErrWaitingApproval) {
			break
		}
		if attempt < maxRetries {
			backoff := o.calculateBackoff(attempt)
			o.log(ctx, task.ID, &job.ID, "warn", fmt.Sprintf("Workflow failed: %v. Retrying attempt %d of %d in %v...", err, attempt+1, maxRetries, backoff))
			time.Sleep(backoff)
		}
	}

	finalStatus := models.WorkflowJobStatusDone
	var finalErr string
	if err != nil {
		if errors.Is(err, workflow.ErrPaused) || errors.Is(err, workflow.ErrWaitingApproval) {
			finalStatus = models.WorkflowJobStatusPaused
			finalErr = err.Error()
		} else if errors.Is(err, context.Canceled) {
			// Check database state to see if it was paused or cancelled by user
			if latestJob, jErr := o.workflows.LatestByTaskID(context.Background(), task.ID); jErr == nil && latestJob != nil {
				if latestJob.Status == models.WorkflowJobStatusPaused {
					finalStatus = models.WorkflowJobStatusPaused
					finalErr = "workflow paused by user"
				} else {
					finalStatus = models.WorkflowJobStatusFailed
					finalErr = "workflow cancelled by user"
				}
			} else {
				finalStatus = models.WorkflowJobStatusFailed
				finalErr = "workflow context cancelled"
			}
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
		if errors.Is(err, workflow.ErrPaused) || errors.Is(err, workflow.ErrWaitingApproval) || finalStatus == models.WorkflowJobStatusPaused {
			cleanupCtx := context.WithoutCancel(ctx)
			_, _ = o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusPaused, "last_error": finalErr})
			o.log(cleanupCtx, task.ID, &job.ID, "info", finalErr)
			return
		}
		o.fail(ctx, job, fmt.Errorf("%s", finalErr))
		return
	}
	cleanupCtx := context.WithoutCancel(ctx)
	defer o.cleanupWorkspaceAfterFinalState(cleanupCtx, task.ID)
	if _, err := o.workflows.UpdateJob(cleanupCtx, job.ID, map[string]any{"status": models.WorkflowJobStatusDone, "step": models.WorkflowStepDone, "last_error": ""}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	_ = o.checkpoint(cleanupCtx, task.ID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone, "steps": result.Status})
	o.log(cleanupCtx, task.ID, &job.ID, "info", "workflow completed and is waiting for human PR approval")
}

func (o *Orchestrator) calculateBackoff(attempt int) time.Duration {
	// formula: delay = min(2^attempt * 2, 60) seconds
	power := 1 << attempt
	seconds := power * 2
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}


