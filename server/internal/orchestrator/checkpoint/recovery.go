package checkpoint

import (
	"context"
	"fmt"
	"strings"

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
		actualStepID := sc.StepID
		if actualStepID == "" {
			actualStepID = stepID
		}

		if actualStepID != workflow.StepAnalyze {
			// Review and Fix should always run when the current job is Review.
			if jobStep == workflow.StepReview &&
				(actualStepID == workflow.StepReview || actualStepID == workflow.StepFix) {
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

				// Re-apply saved patches for code generation/fix steps.
				if strings.HasPrefix(actualStepID, workflow.StepCodeBackend) ||
					strings.HasPrefix(actualStepID, workflow.StepCodeFrontend) ||
					actualStepID == workflow.StepFix {

					if patch, err := s.GetSavedPatch(ctx, task.ID, actualStepID); err == nil && patch != "" {
						s.Log(
							ctx,
							task.ID,
							nil,
							"info",
							fmt.Sprintf("step %s: re-applying saved patch to workspace", actualStepID),
						)

						worktreeSuffix := ""
						if task.Complexity != models.TaskComplexityEasy {
							if strings.HasPrefix(actualStepID, workflow.StepCodeBackend) {
								worktreeSuffix = "-be-worktree"
							} else if strings.HasPrefix(actualStepID, workflow.StepCodeFrontend) {
								worktreeSuffix = "-fe-worktree"
							}
						}

						if applyPatch != nil {
							if err := applyPatch(
								ctx,
								task,
								agent,
								actualStepID,
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
										actualStepID,
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
		}

		return runner(ctx, sc)
	}
}
