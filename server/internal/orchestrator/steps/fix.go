package steps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecuteFix(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, stepCtx workflow.StepContext) (map[string]any, error) {
	t, err := deps.Tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return map[string]any{"status": "skipped", "info": "skipped fix step for easy task"}, nil
	}

	var prFeedback string
	if checkpoints, cpErr := deps.Workflows.ListCheckpoints(ctx, task.ID); cpErr == nil {
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
	if deps.LLM != nil {
		var diffText string
		if deps.RepoUtil != nil {
			diffText, _ = deps.RepoUtil.CapturePRDiff(ctx, task, agent, "main")
		}

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

		out, err := deps.RunLLMStep(ctx, task, agent, jobID, workflow.StepFix, instruction)
		if err != nil {
			return nil, err
		}
		var patchApplied bool
		if parsed, ok := out["parsed"].(map[string]any); ok {
			p := patch.ExtractPatch(parsed)
			if p != "" {
				_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepFix, "patch", p)
				if deps.RepoUtil != nil {
					if applyErr := deps.RepoUtil.ApplyPatch(ctx, task, agent, workflow.StepFix, p, ""); applyErr != nil {
						return nil, fmt.Errorf("apply patch: %w", applyErr)
					}
				}
				patchApplied = true
			}
		}
		if deps.RepoUtil != nil {
			if diffText, diffErr := deps.RepoUtil.CaptureWorkspaceDiff(ctx, task, agent, workflow.StepFix, ""); diffErr == nil && diffText != "" {
				_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepFix, "diff", diffText)
			}
		}

		if patchApplied {
			var changedFiles []string
			if deps.RepoUtil != nil {
				repoHostPath, err := deps.RepoUtil.GetTaskRepoHostPath(ctx, task)
				if err != nil {
					return nil, err
				}

				var diffErr error
				changedFiles, diffErr = deps.RepoUtil.GetChangedFiles(ctx, task, agent, repoHostPath, "")
				if diffErr != nil {
					deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
				}
			}
			if len(changedFiles) > 0 {
				if _, errT := deps.RunTargetedTests(ctx, task, agent, jobID, "fix_test", changedFiles, ""); errT != nil {
					deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
				}
			}

			if _, err := deps.UpdateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
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
