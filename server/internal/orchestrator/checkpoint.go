package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) getSuccessfulCheckpoint(ctx context.Context, taskID string, step string) (map[string]any, bool) {
	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, false
	}
	var latestSuccess *models.WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step == step {
			var state map[string]any
			if err := json.Unmarshal(cp.State, &state); err == nil {
				if state["status"] == "success" {
					latestSuccess = &cp
					break
				}
			}
		}
	}
	if latestSuccess != nil {
		var state map[string]any
		_ = json.Unmarshal(latestSuccess.State, &state)
		if out, ok := state["output"].(map[string]any); ok {
			return out, true
		}
		return map[string]any{}, true
	}
	return nil, false
}

func (o *Orchestrator) countSuccessfulCheckpoints(ctx context.Context, taskID string, step string) int {
	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return 0
	}
	count := 0
	for _, cp := range checkpoints {
		if cp.Step != step {
			continue
		}
		var state map[string]any
		if err := json.Unmarshal(cp.State, &state); err != nil {
			continue
		}
		if status, _ := state["status"].(string); status == workflow.StepStatusSuccess {
			count++
		}
	}
	return count
}

func (o *Orchestrator) getSavedPatch(ctx context.Context, taskID string, step string) (string, error) {
	if o.artifacts == nil {
		return "", fmt.Errorf("artifacts repository is not configured")
	}
	arts, err := o.artifacts.ListByTaskID(ctx, taskID)
	if err != nil {
		return "", err
	}
	var latestPatch *models.WorkflowArtifact
	for i := len(arts) - 1; i >= 0; i-- {
		art := arts[i]
		if art.Step == step && art.Type == "patch" {
			latestPatch = &art
			break
		}
	}
	if latestPatch == nil {
		return "", fmt.Errorf("no patch artifact found for step %s", step)
	}
	var patch string
	if err := json.Unmarshal(latestPatch.Payload, &patch); err == nil {
		return patch, nil
	}
	return string(latestPatch.Payload), nil
}

func (o *Orchestrator) withCheckpointRecovery(task *models.Task, agent *models.Agent, jobStep string, stepID string, runner workflow.StepFunc) workflow.StepFunc {
	return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		if stepID != workflow.StepAnalyze {
			if jobStep == workflow.StepReview && (stepID == workflow.StepReview || stepID == workflow.StepFix) {
				return runner(ctx, sc)
			}
			if output, exists := o.getSuccessfulCheckpoint(ctx, task.ID, stepID); exists {
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: resuming from previous successful checkpoint", stepID))

				if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix {
					if patch, err := o.getSavedPatch(ctx, task.ID, stepID); err == nil && patch != "" {
						o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: re-applying saved patch to workspace", stepID))

						worktreeSuffix := ""
						if stepID == workflow.StepCodeBackend {
							worktreeSuffix = "-be-worktree"
						} else if stepID == workflow.StepCodeFrontend {
							worktreeSuffix = "-fe-worktree"
						}

						if applyErr := o.applyPatch(ctx, task, agent, stepID, patch, worktreeSuffix); applyErr != nil {
							o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("step %s: failed to re-apply patch (%v), rerunning step", stepID, applyErr))
							return runner(ctx, sc)
						}
					}
				}

				switch stepID {
				case workflow.StepPlan:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusCoding)
				case workflow.StepMerge:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
				case workflow.StepReview:
					nextStatus := models.TaskStatusTesting
					if parsed, ok := output["parsed"].(map[string]any); ok {
						if findings, exists := parsed["findings"]; exists {
							if slice, ok := findings.([]any); ok && len(slice) > 0 {
								nextStatus = models.TaskStatusFixing
							}
						}
					}
					_, _ = o.updateTaskStatus(ctx, task.ID, nextStatus)
				case workflow.StepFix:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
				case workflow.StepTest:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting)
				case workflow.StepPR:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusHumanReview)
				}

				return output, nil
			}
		}
		return runner(ctx, sc)
	}
}

func (o *Orchestrator) saveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	if o.artifacts == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	artifact := &models.WorkflowArtifact{
		JobID:   jobID,
		TaskID:  taskID,
		Step:    step,
		Type:    artType,
		Payload: raw,
	}
	return o.artifacts.Create(ctx, artifact)
}
