package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepFix(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepCtx workflow.StepContext) (map[string]any, error) {
	t, err := o.tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return map[string]any{"status": "skipped", "info": "skipped fix step for easy task"}, nil
	}

	var prFeedback string
	if checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID); cpErr == nil {
		for _, cp := range checkpoints {
			if cp.Step == "pr_rejection" {
				var state map[string]any
				if json.Unmarshal(cp.State, &state) == nil {
					if f, _ := state["feedback"].(string); f != "" {
						prFeedback = f
					}
				}
			}
		}
	}

	if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
		if limitReached, _ := reviewOut["cycle_limit_reached"].(bool); limitReached {
			return map[string]any{
				"status": "skipped",
				"info":   "review-fix cycle limit reached, skipping fix step",
			}, nil
		}
		if prFeedback == "" {
			if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
				if findings, exists := parsed["findings"]; exists {
					if slice, ok := findings.([]any); ok && len(slice) == 0 {
						return map[string]any{
							"status": "skipped",
							"info":   "no review findings, skipped fix step",
						}, nil
					}
				}
			}
		}
	}
	if o.llm != nil {
		diffText, _ := o.capturePRDiff(ctx, task, agent, "main")

		instruction := "Fix review findings. Here is the current workspace diff:\n\n" + diffText + "\n\n"

		var findingsJSON string
		if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
			if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
				if findings, exists := parsed["findings"]; exists {
					if findingsBytes, err := json.MarshalIndent(findings, "", "  "); err == nil {
						findingsJSON = string(findingsBytes)
					}
				}
			}
		}

		if prFeedback != "" {
			instruction += fmt.Sprintf("Fix review findings and address the following PR rejection feedback:\n\n%s\n\n", prFeedback)
		} else if findingsJSON != "" {
			instruction += fmt.Sprintf("Address the following review findings:\n\n%s\n\n", findingsJSON)
		}
		instruction += "IMPORTANT: The diff above shows the current proposed changes. Your patch must apply to the files AS THEY ARE AFTER the diff is applied. DO NOT recreate files that the diff already creates. Generate an incremental patch that fixes ONLY the findings.\n\n"
		instruction += "Return JSON with fixes_applied, files_changed, and patch text when available."

		out, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepFix, instruction)
		if err != nil {
			return nil, err
		}
		var patchApplied bool
		if parsed, ok := out["parsed"].(map[string]any); ok {
			patch := patch.ExtractPatch(parsed)
			if patch != "" {
				_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "patch", patch)
				if applyErr := o.applyPatch(ctx, task, agent, workflow.StepFix, patch, ""); applyErr != nil {
					return nil, fmt.Errorf("apply patch: %w", applyErr)
				}
				patchApplied = true
			}
		}
		if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepFix, ""); diffErr == nil && diffText != "" {
			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "diff", diffText)
		}

		if patchApplied {
			repoHostPath, err := o.getTaskRepoHostPath(ctx, task)
			if err != nil {
				return nil, err
			}

			changedFiles, diffErr := o.getChangedFiles(ctx, task, agent, repoHostPath, "")
			if diffErr != nil {
				o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
			}
			if len(changedFiles) > 0 {
				if _, errT := o.runTargetedTests(ctx, task, agent, jobID, "fix_test", changedFiles, ""); errT != nil {
					o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
				}
			}

			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
				return nil, err
			}
			// We don't delete review & fix checkpoints here anymore; they are skipped when resuming
			// using the job.Step filter in orchestrator_worker.go to preserve cycle counts in DB.
			return nil, workflow.ErrReviewFixLoop
		}

		return map[string]any{
			"status": "success",
			"info":   "no fixes applied",
		}, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
