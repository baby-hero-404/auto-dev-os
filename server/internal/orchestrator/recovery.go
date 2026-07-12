package orchestrator

import (
	"context"
	"errors"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) fail(ctx context.Context, job *models.WorkflowJob, err error) {
	if ctx.Err() == context.Canceled || (err != nil && (errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "signal: killed"))) {
		return
	}
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
