package workflow

import (
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// ValidWorkflowJobTransitions defines allowed transitions for a workflow job,
// covering every UpdateJob status write found in orchestrator/worker.go,
// orchestrator.go, and pr_sync.go (audited exhaustively — retries create a
// fresh queued job row via Enqueue rather than transitioning a failed one,
// so "failed" has no outgoing UpdateJob transitions).
var ValidWorkflowJobTransitions = map[string][]string{
	models.WorkflowJobStatusQueued:  {models.WorkflowJobStatusRunning, models.WorkflowJobStatusFailed},
	models.WorkflowJobStatusRunning: {models.WorkflowJobStatusPaused, models.WorkflowJobStatusDone, models.WorkflowJobStatusFailed, models.WorkflowJobStatusQueued},
	models.WorkflowJobStatusPaused:  {models.WorkflowJobStatusRunning, models.WorkflowJobStatusQueued, models.WorkflowJobStatusFailed, models.WorkflowJobStatusDone},
	models.WorkflowJobStatusDone:    {},
	models.WorkflowJobStatusFailed:  {},
}

// ValidateJobTransition verifies if a workflow job status transition is allowed.
func ValidateJobTransition(from, to string) error {
	if from == to {
		return nil
	}
	allowed, ok := ValidWorkflowJobTransitions[from]
	if !ok {
		return fmt.Errorf("unknown workflow job status: %s", from)
	}
	for _, status := range allowed {
		if status == to {
			return nil
		}
	}
	return fmt.Errorf("invalid workflow job transition from %q to %q", from, to)
}

// ValidateTaskTransition verifies if a task status transition is allowed.
func ValidateTaskTransition(from, to string) error {
	if from == "" {
		from = models.TaskStatusTodo
	}
	if from == to {
		return nil
	}
	allowed, ok := models.ValidTaskTransitions[from]
	if !ok {
		return fmt.Errorf("unknown task status: %s", from)
	}
	for _, status := range allowed {
		if status == to {
			return nil
		}
	}
	return fmt.Errorf("invalid task transition from %q to %q", from, to)
}
