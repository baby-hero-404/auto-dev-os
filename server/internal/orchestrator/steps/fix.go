package steps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// FixStep implements Step for fixing findings/feedback from PR review.
type FixStep struct {
	rt          StepRuntime
	tasks       TaskReader
	checkpoints CheckpointLister
	llm         LLMRunner
	diff        DiffCapturer
	artifacts   ArtifactSaver
	patch       PatchApplier
	tests       TestRunner
	status      StatusUpdater
	log         Logger
}

func NewFixStep(
	rt StepRuntime,
	tasks TaskReader,
	checkpoints CheckpointLister,
	llm LLMRunner,
	diff DiffCapturer,
	artifacts ArtifactSaver,
	patch PatchApplier,
	tests TestRunner,
	status StatusUpdater,
	log Logger,
) *FixStep {
	return &FixStep{
		rt:          rt,
		tasks:       tasks,
		checkpoints: checkpoints,
		llm:         llm,
		diff:        diff,
		artifacts:   artifacts,
		patch:       patch,
		tests:       tests,
		status:      status,
		log:         log,
	}
}

func (s *FixStep) ID() string { return workflow.StepFix }

func (s *FixStep) StatusOnResume(_ StepResult) string {
	return models.TaskStatusReviewing
}

func (s *FixStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	t, err := s.tasks.GetByID(ctx, s.rt.Task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return StepResult{"status": "skipped", "info": "skipped fix step for easy task"}, nil
	}

	var prFeedback string
	if s.checkpoints != nil {
		if checkpoints, cpErr := s.checkpoints.ListCheckpoints(ctx, s.rt.Task.ID); cpErr == nil {
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
	}

	if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
		if limitReached, _ := reviewOut["cycle_limit_reached"].(bool); limitReached {
			return StepResult{
				"status": "skipped",
				"info":   "review-fix cycle limit reached, skipping fix step",
			}, nil
		}
		if prFeedback == "" {
			if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
				findings := getReviewFindings(parsed)
				if findings != nil {
					if !hasActionableFindings(findings) {
						return StepResult{
							"status": "skipped",
							"info":   "no review findings, skipped fix step",
						}, nil
					}
				}
			}
		}
	}
	if s.llm != nil {
		var diffText string
		if s.diff != nil {
			var err error
			diffText, err = s.diff.CapturePRDiff(ctx, s.rt.Task, s.rt.Agent, "main")
			if err != nil || diffText == "" {
				suffix := ""
				if s.rt.Agent != nil {
					if s.rt.Agent.Role == models.AgentRoleBackend {
						suffix = "-be-worktree"
					} else if s.rt.Agent.Role == models.AgentRoleFrontend {
						suffix = "-fe-worktree"
					}
				}
				diffText, _ = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepFix, suffix)
			}
		}
		if diffText == "" && s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "no diff was provided to fix step")
		}

		instruction := "Fix review findings. Here is the current workspace diff:\n\n" + diffText + "\n\n"

		var findingsJSON string
		if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
			if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
				findings := getReviewFindings(parsed)
				if findings != nil {
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
		instruction += "IMPORTANT: The diff above shows the current proposed changes. Your patch must apply to the files AS THEY ARE AFTER the diff is applied. DO NOT recreate files that the diff already creates. Generate an incremental patch that fixes ONLY the findings.\n"
		
		repoContext := ""
		if s.rt.Task.RepositoryID != nil {
			repoContext = "\nIMPORTANT: Your workspace root IS the repository root.\nAll file paths MUST be relative (e.g., internal/model/commit.go).\nDo NOT prefix with the repository name.\nYour diff paths MUST be relative to the repository root, e.g., --- a/filepath. DO NOT include the repository name in the path."
		} else {
			repoContext = "Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath) because you are working in a multi-repository workspace."
		}
		instruction += "IMPORTANT: For the patch text, you MUST generate a valid Unified Diff. Ensure that your hunk headers (@@) have the exact correct line counts matching the target file. " + repoContext + "\n\n"
		instruction += "Return JSON with fixes_applied, files_changed, and patch text when available."

		var out map[string]any
		var err error
		maxRetries := 3
		var patchApplied bool

		for attempt := 1; attempt <= maxRetries; attempt++ {
			out, err = s.llm.RunLLMStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, workflow.StepFix, instruction)
			if err != nil {
				return nil, err
			}

			retryNeeded := false
			if parsed, ok := out["parsed"].(map[string]any); ok {
				p := patch.ExtractPatch(parsed)
				if p != "" {
					if s.patch != nil {
						// Validate
						validationErrs := s.patch.Validate(ctx, s.rt.Task, p, "")
						if len(validationErrs) > 0 {
							if attempt < maxRetries {
								errMsg := ""
								for _, ve := range validationErrs {
									errMsg += "- " + ve.Error() + "\n"
								}
								instruction += fmt.Sprintf("\n\nYour previous patch failed validation. Please fix the following errors:\n%s\nOutput the patch again.", errMsg)
								if s.log != nil {
									s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("Patch validation failed (attempt %d/%d). Retrying...", attempt, maxRetries))
								}
								retryNeeded = true
							} else {
								if s.log != nil {
									s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "Patch validation failed after max retries")
								}
								out["patch_apply_error"] = "Patch validation failed: " + validationErrs[0].Error()
							}
						} else {
							// Validation passed
							if s.artifacts != nil {
								_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepFix, "patch", p)
							}
							if applyErr := s.patch.ApplyPatch(ctx, s.rt.Task, s.rt.Agent, workflow.StepFix, p, ""); applyErr != nil {
								if s.log != nil {
									s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to apply patch generated by LLM (attempt %d/%d): %v", attempt, maxRetries, applyErr))
								}
								
								if pErr, ok := applyErr.(*patch.PolicyViolationError); ok {
									if pErr.Severity == patch.SeverityCritical {
										if s.log != nil {
											s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", fmt.Sprintf("CRITICAL Security Boundary Violation: %s. Pausing task for human review.", pErr.ErrorMsg))
										}
										return nil, fmt.Errorf("%w: %s", workflow.ErrPaused, pErr.ErrorMsg)
									}
									if attempt >= 2 {
										if s.log != nil {
											s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", fmt.Sprintf("Repeated Execution Boundary Violation (attempt %d): %s. Pausing task.", attempt, pErr.ErrorMsg))
										}
										return nil, fmt.Errorf("%w: repeated boundary violations: %s", workflow.ErrPaused, pErr.ErrorMsg)
									}

									jsonErrBytes, _ := json.MarshalIndent(pErr, "", "  ")
									instruction += fmt.Sprintf("\n\nYour previous patch failed to apply due to an Execution Boundary Violation:\n```json\n%s\n```\nPlease regenerate your patch without modifying files outside the execution boundary.", string(jsonErrBytes))
									retryNeeded = true
									out["patch_apply_error"] = pErr.Error()
									continue
								}

								out["patch_apply_error"] = applyErr.Error()
								if attempt < maxRetries {
									instruction += fmt.Sprintf("\n\nYour previous patch failed to apply with error:\n%v\nPlease output a corrected patch that applies cleanly.", applyErr)
									retryNeeded = true
								} else {
									if s.log != nil {
										s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", "Patch apply failed after max retries")
									}
									return nil, fmt.Errorf("failed to apply patch: %w", applyErr)
								}
							} else {
								// Patch applied cleanly. Now let's run targeted tests to verify!
								if s.tests != nil && s.diff != nil {
									if repoHostPath, err := s.diff.GetTaskRepoHostPath(ctx, s.rt.Task); err == nil {
										if changedFiles, diffErr := s.diff.GetChangedFiles(ctx, s.rt.Task, s.rt.Agent, repoHostPath, ""); diffErr == nil && len(changedFiles) > 0 {
											if _, errT := s.tests.RunTargetedTests(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, "fix_test", changedFiles, ""); errT != nil {
												if s.log != nil {
													s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("targeted tests failed (attempt %d/%d): %v", attempt, maxRetries, errT))
												}
												if attempt < maxRetries {
													instruction += fmt.Sprintf("\n\nYour patch applied successfully, but the automated tests failed with the following error:\n%v\nPlease analyze the test failure and output a new patch that fixes this issue.", errT)
													retryNeeded = true
												} else {
													if s.log != nil {
														s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", "Targeted tests failed after max retries")
													}
													return nil, fmt.Errorf("targeted tests failed: %w", errT)
												}
											} else {
												patchApplied = true
											}
										} else {
											patchApplied = true
										}
									} else {
										patchApplied = true
									}
								} else {
									patchApplied = true
								}
							}
						}
					}
				}
			}
			if !retryNeeded {
				break
			}
		}
		if s.diff != nil {
			if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepFix, ""); diffErr == nil && diffText != "" {
				if s.artifacts != nil {
					_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepFix, "diff", diffText)
				}
			}
		}

		if patchApplied {
			if s.status != nil {
				if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusReviewing); err != nil {
					return nil, err
				}
			}
			// We don't delete review & fix checkpoints here anymore; they are skipped when resuming
			// using the job.Step filter in orchestrator_worker.go to preserve cycle counts in DB.
			return nil, workflow.ErrReviewFixLoop
		}

		return StepResult{
			"status": "success",
			"info":   "no fixes applied",
		}, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
