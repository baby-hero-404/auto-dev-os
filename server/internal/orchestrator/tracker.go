package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
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
	updated, err := o.tasks.Update(ctx, taskID, models.UpdateTaskInput{Status: &newStatus})
	if err != nil {
		return nil, err
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

	return updated, nil
}
