package checkpoint

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (s *Store) WithCheckpointRecovery(
	task *models.Task,
	agent *models.Agent,
	jobStep string,
	stepID string,
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
						if stepID == workflow.StepCodeBackend {
							worktreeSuffix = "-be-worktree"
						} else if stepID == workflow.StepCodeFrontend {
							worktreeSuffix = "-fe-worktree"
						}

						if applyPatch != nil {
							if applyErr := applyPatch(ctx, task, agent, stepID, patch, worktreeSuffix); applyErr != nil {
								s.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("step %s: failed to re-apply patch (%v), rerunning step", stepID, applyErr))
								return runner(ctx, sc)
							}
						}
					}
				}

				if updateTaskStatus != nil {
					switch stepID {
					case workflow.StepPlan:
						_, _ = updateTaskStatus(ctx, task.ID, models.TaskStatusCoding)
					case workflow.StepMerge:
						_, _ = updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
					case workflow.StepReview:
						nextStatus := models.TaskStatusTesting
						if parsed, ok := output["parsed"].(map[string]any); ok {
							if findings, exists := parsed["findings"]; exists {
								if slice, ok := findings.([]any); ok && len(slice) > 0 {
									nextStatus = models.TaskStatusFixing
								}
							}
						}
						_, _ = updateTaskStatus(ctx, task.ID, nextStatus)
					case workflow.StepFix:
						_, _ = updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
					case workflow.StepTest:
						_, _ = updateTaskStatus(ctx, task.ID, models.TaskStatusTesting)
					case workflow.StepPR:
						_, _ = updateTaskStatus(ctx, task.ID, models.TaskStatusHumanReview)
					}
				}

				return output, nil
			}
		}
		return runner(ctx, sc)
	}
}
