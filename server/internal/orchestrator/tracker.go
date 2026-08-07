package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) checkpoint(ctx context.Context, taskID string, jobID *string, step string, state map[string]any) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return o.workflows.CreateCheckpoint(ctx, models.WorkflowCheckpoint{TaskID: taskID, JobID: jobID, Step: step, State: raw})
}

func (o *Orchestrator) log(ctx context.Context, taskID string, jobID *string, level, message string) {
	if o.workflows == nil {
		slog.Warn(message, "task_id", taskID, "level", level)
		return
	}
	stepID, hasStep := ctx.Value("workflow_step_id").(string)
	attempt, hasAttempt := ctx.Value("workflow_attempt").(int)
	if hasStep && stepID != "" {
		if hasAttempt {
			message = fmt.Sprintf("[%s #%d] %s", stepID, attempt, message)
		} else {
			message = fmt.Sprintf("[%s] %s", stepID, message)
		}
	} else if hasAttempt {
		message = fmt.Sprintf("[#%d] %s", attempt, message)
	}
	message = redactSecrets(message)
	if err := o.workflows.CreateLog(ctx, models.TaskLog{TaskID: taskID, JobID: jobID, Level: level, Message: message}); err != nil {
		slog.Warn("persist workflow log failed", observability.LogAttrs(ctx, "task_id", taskID, "job_id", jobID, "level", level, "error", err)...)
	}
	if o.eventAdapter != nil {
		if err := o.eventAdapter.ProcessLine(context.WithoutCancel(ctx), taskID, message); err != nil {
			slog.Warn("task event adapter failed", observability.LogAttrs(ctx, "task_id", taskID, "error", err)...)
		}
	}
	o.checkEventVolumeGuardrail(context.WithoutCancel(ctx), taskID)
	switch level {
	case "error":
		observability.Error(ctx, message, "job_id", jobID)
	case "warn":
		observability.Warn(ctx, message, "job_id", jobID)
	default:
		observability.Info(ctx, message, "job_id", jobID)
	}
}

// recordLearnedSkillOutcome updates usage/success counters (REQ-003) for
// whichever learned skills context_load recorded as loaded (checkpoint
// step "context_load", state key "skills_loaded") for this task. Best-effort:
// checkpoint lookup/parse failures are logged, never propagated.
func (o *Orchestrator) recordLearnedSkillOutcome(ctx context.Context, task *models.Task, success bool) {
	checkpoints, err := o.workflows.ListCheckpoints(ctx, task.ID)
	if err != nil {
		slog.Warn("learned-skill outcome: failed to list checkpoints", "task_id", task.ID, "error", err)
		return
	}
	var skillIDs []string
	for _, cp := range checkpoints {
		if cp.Step != "context_load" {
			continue
		}
		var state struct {
			SkillsLoaded []string `json:"skills_loaded"`
		}
		if err := json.Unmarshal(cp.State, &state); err != nil {
			continue
		}
		skillIDs = state.SkillsLoaded
	}
	if len(skillIDs) == 0 {
		return
	}
	o.learnEngine.RecordSkillOutcome(context.WithoutCancel(ctx), skillIDs, success)
}

// applyGuardrails implements the execution guardrails
// (docs/openspecs/status-driven-agent-workspace/tasks.md 2.3) at the single
// choke point every task status transition passes through. It tracks
// RetryCount (increment on fixing<-testing/reviewing re-entry, reset on a
// successful testing exit) and ExecutionStartedAt (set once, on first leaving
// TaskStatusTodo), then — unless *newStatus already targets a terminal/
// blocked status — checks the max-retries and execution-timeout guardrails;
// on a trip it rewrites *newStatus to TaskStatusBlocked, sets BlockReason,
// and best-effort emits the corresponding task.error event.
func (o *Orchestrator) applyGuardrails(ctx context.Context, task *models.Task, newStatus *string) *models.UpdateTaskInput {
	input := &models.UpdateTaskInput{}

	retryCount := task.RetryCount
	if *newStatus == models.TaskStatusFixing && (task.Status == models.TaskStatusTesting || task.Status == models.TaskStatusReviewing) {
		retryCount++
	}
	if task.Status == models.TaskStatusTesting && (*newStatus == models.TaskStatusPrReady || *newStatus == models.TaskStatusMerged || *newStatus == models.TaskStatusHumanReview) {
		retryCount = 0
	}
	if retryCount != task.RetryCount {
		input.RetryCount = &retryCount
	}

	executionStartedAt := task.ExecutionStartedAt
	if task.Status == models.TaskStatusTodo && executionStartedAt == nil {
		now := time.Now()
		executionStartedAt = &now
		input.ExecutionStartedAt = &now
	}

	blockReason := ""
	if *newStatus != models.TaskStatusBlocked && *newStatus != models.TaskStatusFailed && *newStatus != models.TaskStatusMerged {
		maxRetries, maxMinutes := 5, 120
		if o.projects != nil {
			if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
				if p.MaxTaskRetryCount > 0 {
					maxRetries = p.MaxTaskRetryCount
				}
				if p.MaxExecutionMinutes > 0 {
					maxMinutes = p.MaxExecutionMinutes
				}
			}
		}
		switch {
		case service.EvalMaxRetries(retryCount, maxRetries):
			blockReason = service.GuardrailReasonMaxRetries
		case service.EvalExecutionTimeout(executionStartedAt, time.Now(), maxMinutes):
			blockReason = service.GuardrailReasonExecutionTime
		}
	}
	if blockReason != "" {
		*newStatus = models.TaskStatusBlocked
		if o.eventAdapter != nil {
			if err := o.eventAdapter.EmitError(context.WithoutCancel(ctx), task.ID, blockReason, true); err != nil {
				slog.Warn("failed to emit guardrail task.error event", observability.LogAttrs(ctx, "task_id", task.ID, "reason", blockReason, "error", err)...)
			}
		}
	}
	input.BlockReason = &blockReason
	return input
}

// checkSecurityReview implements the security/deny-list guardrail (tasks.md
// 2.3), injected into repoutil.Manager so every patch.Runner it builds
// blocks LLM-generated diffs that touch deny-listed paths or contain
// hardcoded-secret patterns before they're applied. Uses the default deny
// list (docs/openspecs/status-driven-agent-workspace/tasks.md's
// "project-configurable deny-list" is deferred — Project has no such field
// yet — this is a deliberate v1 scope reduction consistent with "keep it
// minimal").
func (o *Orchestrator) checkSecurityReview(changedPaths []string, diffContent string) (bool, string) {
	return service.EvalSecurityReview(changedPaths, service.DefaultSecurityDenyListPaths, diffContent)
}

// checkEventVolumeGuardrail is the event_volume_exceeded guardrail
// (tasks.md 2.3): checked after every task_events row is written (via
// eventAdapter.ProcessLine, one call up in log()) rather than on a cheap
// interval — simplest-correct first; if per-event CountByTaskID proves too
// hot in practice, batch/interval it then. Best-effort: failures here never
// interrupt the calling workflow step.
func (o *Orchestrator) checkEventVolumeGuardrail(ctx context.Context, taskID string) {
	if o.taskEventCounter == nil {
		return
	}
	count, err := o.taskEventCounter.CountByTaskID(ctx, taskID)
	if err != nil {
		return
	}
	maxEvents := 20000
	if o.projects != nil {
		if task, err := o.tasks.GetByID(ctx, taskID); err == nil {
			if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.MaxEventCount > 0 {
				maxEvents = p.MaxEventCount
			}
		}
	}
	if !service.EvalEventVolume(int(count), maxEvents) {
		return
	}
	if err := o.blockTask(ctx, taskID, service.GuardrailReasonEventVolume); err != nil {
		slog.Warn("failed to block task on event volume guardrail", observability.LogAttrs(ctx, "task_id", taskID, "error", err)...)
	}
}

// blockTask forces taskID into TaskStatusBlocked with the given BlockReason,
// used by guardrails (event volume, security review) that trip outside the
// normal updateTaskStatus call path. A no-op if the task is already
// terminal/blocked.
func (o *Orchestrator) blockTask(ctx context.Context, taskID, reason string) error {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status == models.TaskStatusBlocked || task.Status == models.TaskStatusFailed || task.Status == models.TaskStatusMerged {
		return nil
	}
	if err := workflow.ValidateTaskTransition(task.Status, models.TaskStatusBlocked); err != nil {
		return err
	}
	blocked := models.TaskStatusBlocked
	if _, err := o.tasks.Update(ctx, taskID, models.UpdateTaskInput{Status: &blocked, BlockReason: &reason}); err != nil {
		return err
	}
	if o.eventAdapter != nil {
		if err := o.eventAdapter.EmitError(context.WithoutCancel(ctx), taskID, reason, true); err != nil {
			slog.Warn("failed to emit guardrail task.error event", observability.LogAttrs(ctx, "task_id", taskID, "reason", reason, "error", err)...)
		}
	}
	return nil
}

func (o *Orchestrator) updateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	input := o.applyGuardrails(ctx, task, &newStatus)

	if err := workflow.ValidateTaskTransition(task.Status, newStatus); err != nil {
		return nil, fmt.Errorf("invalid task status transition from %q to %q: %w", task.Status, newStatus, err)
	}
	input.Status = &newStatus
	updated, err := o.tasks.Update(ctx, taskID, *input)
	if err != nil {
		return nil, err
	}

	if o.learnEngine != nil && (newStatus == models.TaskStatusMerged || newStatus == models.TaskStatusFailed) {
		o.recordLearnedSkillOutcome(ctx, updated, newStatus == models.TaskStatusMerged)
	}
	if o.learnEngine != nil && newStatus == models.TaskStatusMerged {
		autonomous := false
		if o.projects != nil {
			if proj, pErr := o.projects.GetByID(ctx, updated.ProjectID); pErr == nil && proj != nil {
				autonomous = proj.DefaultAutonomy == "autonomous"
			}
		}
		leCtx := context.WithoutCancel(ctx)
		go o.learnEngine.ExtractLearnedSkills(leCtx, updated, autonomous)
	}

	if o.wkspace != nil {
		if ws, wsErr := o.wkspace.LoadTaskWorkspace(ctx, updated); wsErr == nil && ws != nil {
			taskSnap := models.TaskStateSnapshot{
				TaskID:      updated.ID,
				ProjectID:   updated.ProjectID,
				Title:       updated.Title,
				Description: updated.Description,
				Status:      updated.Status,
				Complexity:  updated.Complexity,
				SpecStatus:  updated.SpecStatus,
				Labels:      updated.Labels,
			}
			taskJSONPath := filepath.Join(ws.Root, "task.json")
			if taskBytes, err := json.MarshalIndent(taskSnap, "", "  "); err == nil {
				_ = os.WriteFile(taskJSONPath, taskBytes, 0o644)
			}
		}
	}

	if updated.ParentTaskID != nil && (newStatus == models.TaskStatusMerged || newStatus == models.TaskStatusFailed) {
		o.onChildTaskTerminal(ctx, updated, newStatus == models.TaskStatusMerged)
	}

	return updated, nil
}
