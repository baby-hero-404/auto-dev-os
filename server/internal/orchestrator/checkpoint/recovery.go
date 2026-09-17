package checkpoint

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// ResumeStatusFunc returns the task status to restore when a step is
// skipped during checkpoint recovery. Return "" for no status transition.
type ResumeStatusFunc func(output map[string]any) string

// WithCheckpointRecovery wraps a CLI step runner so that a workflow retry
// skips re-running a step that already succeeded on a previous pass,
// returning its cached checkpoint output and restoring the task status the
// caller expects for that resume point instead.
//
// The patch-reapply and analyze/review/fix special-casing this used to carry
// for the deleted API-native DAG (which saved unified-diff patches per code
// step and needed to re-apply them into the workspace on resume) is gone:
// CLI steps work against git worktrees directly, so there is nothing to
// reapply — a resumed CLI step's work is already committed there.
func (s *Store) WithCheckpointRecovery(
	stepID string,
	statusOnResume ResumeStatusFunc,
	task *models.Task,
	runner workflow.StepFunc,
	updateTaskStatus func(
		ctx context.Context,
		taskID string,
		newStatus string,
	) (*models.Task, error),
) workflow.StepFunc {
	return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		actualStepID := sc.StepID
		if actualStepID == "" {
			actualStepID = stepID
		}

		if models.IsStateMachineEnabled(ctx) {
			return runner(ctx, sc)
		}

		if output, exists := s.GetSuccessful(ctx, task.ID, actualStepID); exists {
			s.Log(
				ctx,
				task.ID,
				nil,
				"info",
				fmt.Sprintf("step %s: resuming from previous successful checkpoint", actualStepID),
			)

			if updateTaskStatus != nil && statusOnResume != nil {
				if resumeStatus := statusOnResume(output); resumeStatus != "" {
					if _, err := updateTaskStatus(ctx, task.ID, resumeStatus); err != nil {
						s.Log(
							ctx,
							task.ID,
							nil,
							"warn",
							fmt.Sprintf(
								"step %s: failed to restore status %s on resume: %v",
								actualStepID,
								resumeStatus,
								err,
							),
						)
					}
				}
			}

			return output, nil
		}

		return runner(ctx, sc)
	}
}
