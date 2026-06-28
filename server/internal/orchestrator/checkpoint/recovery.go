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
	applyPatch func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, patchText string, worktreeSuffix string) error,
	updateTaskStatus func(ctx context.Context, taskID string, newStatus string) (*models.Task, error),
) workflow.StepFunc {
	return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		if stepID != workflow.StepAnalyze {
			if jobStep == workflow.StepReview && (stepID == workflow.StepReview || stepID == workflow.StepFix) {
				return runner(ctx, sc)
			}
			if output, exists := s.GetSuccessful(ctx, task.ID, stepID); exists {
				s.Log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: resuming from previous successful checkpoint", stepID))

				if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix {
					if patch, err := s.GetSavedPatch(ctx, task.ID, stepID); err == nil && patch != "" {
						s.Log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: re-applying saved patch to workspace", stepID))

						worktreeSuffix := ""
						if task.Complexity != models.TaskComplexityEasy {
							if stepID == workflow.StepCodeBackend {
								worktreeSuffix = "-be-worktree"
							} else if stepID == workflow.StepCodeFrontend {
								worktreeSuffix = "-fe-worktree"
							}
						}

						if applyPatch != nil {
							if applyErr := applyPatch(ctx, task, agent, stepID, patch, worktreeSuffix); applyErr != nil {
								s.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("step %s: failed to re-apply patch (%v), rerunning step", stepID, applyErr))
								return runner(ctx, sc)
							}
						}
					}
				}

				return output, nil
			}
		}
		return runner(ctx, sc)
	}
}
