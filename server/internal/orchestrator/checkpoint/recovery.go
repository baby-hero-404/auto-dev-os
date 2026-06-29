package checkpoint

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// ResumeStatusFunc returns the task status to restore when a step is
// skipped during checkpoint recovery. It receives the cached checkpoint
// output so that steps like review can choose between "testing" and
// "fixing" based on findings. Return "" for no status transition.
type ResumeStatusFunc func(output map[string]any) string

func (s *Store) WithCheckpointRecovery(
	stepID string,
	statusOnResume ResumeStatusFunc,
	task *models.Task,
	agent *models.Agent,
	jobStep string,
	runner workflow.StepFunc,
	applyPatch func(
		ctx context.Context,
		task *models.Task,
		agent *models.Agent,
		stepName string,
		patchText string,
		worktreeSuffix string,
	) error,
	updateTaskStatus func(
		ctx context.Context,
		taskID string,
		newStatus string,
	) (*models.Task, error),
) workflow.StepFunc {
	return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		if stepID != workflow.StepAnalyze {
			// Review and Fix should always run when the current job is Review.
			if jobStep == workflow.StepReview &&
				(stepID == workflow.StepReview || stepID == workflow.StepFix) {
				return runner(ctx, sc)
			}

			if output, exists := s.GetSuccessful(ctx, task.ID, stepID); exists {
				s.Log(
					ctx,
					task.ID,
					nil,
					"info",
					fmt.Sprintf("step %s: resuming from previous successful checkpoint", stepID),
				)

				// Re-apply saved patches for code generation/fix steps.
				if stepID == workflow.StepCodeBackend ||
					stepID == workflow.StepCodeFrontend ||
					stepID == workflow.StepFix {

					if patch, err := s.GetSavedPatch(ctx, task.ID, stepID); err == nil && patch != "" {
						s.Log(
							ctx,
							task.ID,
							nil,
							"info",
							fmt.Sprintf("step %s: re-applying saved patch to workspace", stepID),
						)

						worktreeSuffix := ""
						if task.Complexity != models.TaskComplexityEasy {
							switch stepID {
							case workflow.StepCodeBackend:
								worktreeSuffix = "-be-worktree"
							case workflow.StepCodeFrontend:
								worktreeSuffix = "-fe-worktree"
							}
						}

						if applyPatch != nil {
							if err := applyPatch(
								ctx,
								task,
								agent,
								stepID,
								patch,
								worktreeSuffix,
							); err != nil {
								s.Log(
									ctx,
									task.ID,
									nil,
									"warn",
									fmt.Sprintf(
										"step %s: failed to re-apply patch (%v), rerunning step",
										stepID,
										err,
									),
								)
								return runner(ctx, sc)
							}
						}
					}
				}

				// Restore task status when resuming from a checkpoint.
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
									stepID,
									resumeStatus,
									err,
								),
							)
						}
					}
				}

				return output, nil
			}
		}

		return runner(ctx, sc)
	}
}