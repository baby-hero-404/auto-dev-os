package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// patchRetryConfig parameterizes the shared "generate diff -> validate -> apply -> targeted
// test" retry loop used by code_backend, code_frontend, and fix (Issue 3: these three files
// duplicated the same ~130 lines of control flow, only varying by worktree suffix, test
// label, and field names).
type patchRetryConfig struct {
	Task           *models.Task
	Agent          *models.Agent
	JobID          string
	StepID         string
	WorktreeSuffix string
	TestLabel      string
	MaxRetries     int

	// Agentic marks that the LLM call already applied its edits directly via native tool
	// calls (Issue 1+2), so there is no patch/diff text to extract/validate/apply — only the
	// post-hoc targeted-test verification gate still applies.
	Agentic bool

	LLM         LLMRunner
	Worktree    WorktreeManager
	Patcher     PatchApplier
	Diff        DiffCapturer
	Artifacts   ArtifactSaver
	Tester      TestRunner
	Tasks       TaskRepository
	Log         Logger
	Checkpoints CheckpointLister
}

func (cfg patchRetryConfig) logf(ctx context.Context, level, format string, args ...any) {
	if cfg.Log == nil {
		return
	}
	cfg.Log.Log(ctx, cfg.Task.ID, &cfg.JobID, level, fmt.Sprintf(format, args...))
}

// filesReadNote renders a compact "already read" hint for the next retry attempt's
// instruction, so the model doesn't have to re-discover file contents it already read in the
// discarded prior attempt's conversation (Issue 6 retry carry-forward — each outer retry
// rebuilds messages from scratch, so this is the only continuity across attempts).
func filesReadNote(filesRead []string) string {
	if len(filesRead) == 0 {
		return ""
	}
	return fmt.Sprintf("\n\nFor reference, you already read these files in your previous attempt (their content is unlikely to have changed unless you edited them): %s", strings.Join(filesRead, ", "))
}

// runPatchRetryLoop drives the LLM call plus patch validate/apply/targeted-test retry loop
// shared by code_backend, code_frontend, and fix. It returns the last LLM step result and
// whether a patch was successfully applied (and, if tests ran, passed) by the time the loop
// terminated.
func runPatchRetryLoop(ctx context.Context, cfg patchRetryConfig, baseInstruction string) (map[string]any, bool, error) {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var out map[string]any
	var err error
	var patchApplied bool
	var retryErrorMsg string
	var hasEditsApplied bool

	wrapErr := func(e error) error {
		if e == nil {
			return nil
		}
		if !hasEditsApplied && !errors.Is(e, workflow.ErrPaused) && !errors.Is(e, workflow.ErrWaitingApproval) && !llm.IsTransientError(e) {
			return fmt.Errorf("%w: %w", workflow.ErrNoProgress, e)
		}
		return e
	}

	var cumulativeFailedCalls []string
	for attempt := 1; attempt <= maxRetries; attempt++ {
		currentInstruction := baseInstruction
		if attempt >= 3 && !cfg.Agentic {
			currentInstruction += "\n\nCRITICAL: Due to persistent Unified Diff failures, you MUST now output your changes in SEARCH/REPLACE block format instead of a Unified Diff. Do NOT output a unified diff patch. Use the following format for each modification:\n<<<<<<< SEARCH\n[exact original code lines here]\n=======\n[replacement code lines here]\n>>>>>>> REPLACE\n"
		}
		if retryErrorMsg != "" {
			currentInstruction += retryErrorMsg
		}
		if attempt >= 2 && cfg.Worktree != nil && !cfg.Agentic {
			cfg.logf(ctx, "info", "Resetting worktree before retry attempt %d", attempt)
			if errReset := cfg.Worktree.ResetRoleWorktrees(ctx, cfg.Task, cfg.Agent, cfg.WorktreeSuffix); errReset != nil {
				cfg.logf(ctx, "error", "failed to reset worktree: %v", errReset)
				return nil, false, wrapErr(fmt.Errorf("worktree corrupted: failed to reset worktree: %w", errReset))
			}
		}
		llmCtx := ctx
		if attempt >= 2 {
			llmCtx = prompts.WithRetry(llmCtx)
		}
		if attempt >= 3 && !cfg.Agentic {
			llmCtx = prompts.WithSearchReplace(llmCtx)
		}
		out, err = cfg.LLM.RunLLMStep(llmCtx, cfg.Task, cfg.Agent, cfg.JobID, cfg.StepID, currentInstruction)
		if err != nil {
			return nil, false, wrapErr(err)
		}

		retryNeeded := false

		if fc, ok := out["failed_calls"].([]string); ok && len(fc) > 0 {
			for _, call := range fc {
				found := false
				for _, existing := range cumulativeFailedCalls {
					if existing == call {
						found = true
						break
					}
				}
				if !found {
					cumulativeFailedCalls = append(cumulativeFailedCalls, call)
				}
			}
		}

		if cfg.Agentic {
			filesReadPrevAttempt, _ := out["files_read"].([]string)

			if toolLoopPartial, _ := out["tool_loop_partial"].(bool); toolLoopPartial {
				// The tool loop exhausted its iteration budget, but real edits already landed
				// in the workspace — salvage them: snapshot the worktree BEFORE testing so a
				// hung/corrupting test run can be reverted to the salvaged state instead of
				// losing the edits or leaving the worktree undefined (Issue 6).
				editsApplied, _ := out["edits_applied"].([]string)
				if len(editsApplied) > 0 {
					hasEditsApplied = true
				}
				cfg.logf(ctx, "warn", "tool loop exhausted its iteration budget but %d edit(s) were applied; attempting to salvage as a partial result (attempt %d/%d)", len(editsApplied), attempt, maxRetries)

				var salvageHash string
				var useGitSalvage = !models.IsStateMachineEnabled(ctx)
				if useGitSalvage && cfg.Worktree != nil {
					ckResult, ckErr := cfg.Worktree.CreateGitCheckpoint(ctx, cfg.Task, cfg.Agent, cfg.StepID+"_salvage", cfg.WorktreeSuffix)
					if ckErr != nil {
						cfg.logf(ctx, "error", "failed to create salvage checkpoint before testing partial result: %v", ckErr)
					} else if ckResult != nil {
						if ckResult.IsEmpty {
							cfg.logf(ctx, "warn", "salvage checkpoint is empty, treating partial result as no progress")
							hasEditsApplied = false
						} else {
							salvageHash = ckResult.Hash
						}
					}
				}

				if !hasEditsApplied {
					if attempt < maxRetries {
						retryErrorMsg = "\n\nYour previous attempt ran out of iterations without making any changes. Please make sure to use edit tools to apply changes."
						retryErrorMsg += filesReadNote(filesReadPrevAttempt)
						retryNeeded = true
					} else {
						cfg.logf(ctx, "error", "Tool loop exhausted iterations with no edits applied after max retries")
						return nil, false, wrapErr(fmt.Errorf("tool loop exhausted iterations with no progress made"))
					}
				} else {
					testsOK := true
					if cfg.Tester != nil && cfg.Diff != nil {
						if repoHostPath, errRP := cfg.Diff.GetTaskRepoHostPath(ctx, cfg.Task); errRP == nil {
							if changedFiles, diffErr := cfg.Diff.GetChangedFiles(ctx, cfg.Task, cfg.Agent, repoHostPath, cfg.WorktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
								if _, errT := cfg.Tester.RunTargetedTests(ctx, cfg.Task, cfg.Agent, cfg.JobID, cfg.TestLabel, changedFiles, cfg.WorktreeSuffix); errT != nil {
									testsOK = false
									cfg.logf(ctx, "warn", "targeted tests failed on salvaged partial result (attempt %d/%d): %v", attempt, maxRetries, errT)
									if useGitSalvage {
										if salvageHash != "" && cfg.Worktree != nil {
											if restoreErr := cfg.Worktree.RestoreGitCheckpoint(ctx, cfg.Task, cfg.Agent, salvageHash, cfg.WorktreeSuffix); restoreErr != nil {
												cfg.logf(ctx, "error", "failed to restore salvage checkpoint after failed test run: %v", restoreErr)
											}
										}
									} else {
										if cfg.Worktree != nil {
											if errReset := cfg.Worktree.ResetRoleWorktrees(ctx, cfg.Task, cfg.Agent, cfg.WorktreeSuffix); errReset != nil {
												cfg.logf(ctx, "error", "failed to reset worktree before snapshot restore: %v", errReset)
											} else if cfg.Checkpoints != nil && cfg.Patcher != nil {
												if snapLister, ok := cfg.Checkpoints.(interface {
													GetLatestExecutionSnapshot(ctx context.Context, taskID string, step string) (*models.ExecutionSnapshot, bool)
												}); ok {
													if snap, exists := snapLister.GetLatestExecutionSnapshot(ctx, cfg.Task.ID, cfg.StepID); exists && snap.WorkspaceDiff != "" {
														if errApply := cfg.Patcher.ApplyPatch(ctx, cfg.Task, cfg.Agent, cfg.StepID+"_restore", snap.WorkspaceDiff, cfg.WorktreeSuffix); errApply != nil {
															cfg.logf(ctx, "error", "failed to restore snapshot diff: %v", errApply)
														} else {
															cfg.logf(ctx, "info", "successfully restored workspace from execution snapshot diff")
														}
													}
												}
											}
										}
									}
									updateAffectedFilesWithErrors(ctx, cfg.Task.ID, cfg.Tasks, cfg.Task, errT)
									if attempt < maxRetries {
										retryErrorMsg = fmt.Sprintf("\n\nYour previous attempt ran out of iterations partway through, and the salvaged partial edits failed automated tests:\n```text\n%v\n```\nPlease continue fixing this.", compressErrorText(errT.Error()))
										retryErrorMsg += filesReadNote(filesReadPrevAttempt)
										retryNeeded = true
									} else {
										cfg.logf(ctx, "error", "Targeted tests failed on salvaged partial result after max retries")
										return nil, false, wrapErr(fmt.Errorf("targeted tests failed on salvaged partial result: %w", errT))
									}
								}
							}
						}
					}
					if testsOK {
						if attempt < maxRetries {
							retryErrorMsg = "\n\nYour previous attempt ran out of iterations partway through. Please continue your work and finish the remaining tasks."
							retryErrorMsg += filesReadNote(filesReadPrevAttempt)
							retryNeeded = true
						} else {
							cfg.logf(ctx, "warn", "Tool loop exhausted iterations and max retries reached. Salvaging as partial success.")
							patchApplied = true
						}
					}
				}
				if !retryNeeded {
					break
				}
				if len(cumulativeFailedCalls) > 0 {
					retryErrorMsg += "\n\nNEGATIVE MEMORY (DO NOT REPEAT):\nThe following tool calls failed in previous attempts. Do NOT try to run these exact same tool calls again, as they will fail. Find an alternative path:\n"
					for _, call := range cumulativeFailedCalls {
						retryErrorMsg += "- " + call + "\n"
					}
				}
				continue
			}

			// Edits were already applied via native tool calls inside the agentic loop; the
			// only remaining gate is the same targeted-test verification used by the
			// diff-based path below. A non-empty 'summary' is required as confirmation that
			// the LLM actually completed the work (mirrors llmrunner.Runner's own agentic
			// completion contract), so a vacuous/empty response isn't mistaken for success.
			parsed, _ := out["parsed"].(map[string]any)
			summary, _ := parsed["summary"].(string)
			if strings.TrimSpace(summary) != "" {
				hasEditsApplied = true
				if cfg.Tester != nil && cfg.Diff != nil {
					if repoHostPath, err := cfg.Diff.GetTaskRepoHostPath(ctx, cfg.Task); err == nil {
						if changedFiles, diffErr := cfg.Diff.GetChangedFiles(ctx, cfg.Task, cfg.Agent, repoHostPath, cfg.WorktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
							if _, errT := cfg.Tester.RunTargetedTests(ctx, cfg.Task, cfg.Agent, cfg.JobID, cfg.TestLabel, changedFiles, cfg.WorktreeSuffix); errT != nil {
								cfg.logf(ctx, "warn", "targeted tests failed (attempt %d/%d): %v", attempt, maxRetries, errT)
								updateAffectedFilesWithErrors(ctx, cfg.Task.ID, cfg.Tasks, cfg.Task, errT)
								if attempt < maxRetries {
									retryErrorMsg = fmt.Sprintf("\n\nYour changes applied successfully, but the automated tests failed with the following error:\n%v\nPlease analyze the test failure and use the available tools to fix it.", errT)
									retryErrorMsg += filesReadNote(filesReadPrevAttempt)
									retryNeeded = true
								} else {
									cfg.logf(ctx, "error", "Targeted tests failed after max retries")
									return nil, false, wrapErr(fmt.Errorf("targeted tests failed: %w", errT))
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
			} else {
				if attempt < maxRetries {
					cfg.logf(ctx, "warn", "LLM response missing summary (attempt %d/%d). Retrying...", attempt, maxRetries)
					retryErrorMsg = "\n\nYou failed to provide the required 'summary' in your final response. You must output a JSON object containing a 'summary' string confirming you have completed the work."
					retryNeeded = true
				} else {
					cfg.logf(ctx, "error", "LLM response missing summary after max retries")
					return nil, false, wrapErr(fmt.Errorf("LLM failed to provide a valid summary of changes"))
				}
			}
			if !retryNeeded {
				break
			}
			if len(cumulativeFailedCalls) > 0 {
				retryErrorMsg += "\n\nNEGATIVE MEMORY (DO NOT REPEAT):\nThe following tool calls failed in previous attempts. Do NOT try to run these exact same tool calls again, as they will fail. Find an alternative path:\n"
				for _, call := range cumulativeFailedCalls {
					retryErrorMsg += "- " + call + "\n"
				}
			}
			continue
		}
		if parsed, ok := out["parsed"].(map[string]any); ok {
			p := patch.ExtractPatch(parsed)
			if p != "" && cfg.Patcher != nil {
				// Validate
				validationErrs := cfg.Patcher.Validate(ctx, cfg.Task, p, cfg.WorktreeSuffix)
				if len(validationErrs) > 0 {
					if attempt < maxRetries {
						errMsg := ""
						for _, ve := range validationErrs {
							errMsg += "- " + ve.Error() + "\n"
						}
						retryErrorMsg = fmt.Sprintf("\n\nYour previous patch failed validation. Please fix the following errors:\n%s\nOutput the patch again.", errMsg)
						cfg.logf(ctx, "warn", "Patch validation failed (attempt %d/%d). Retrying...", attempt, maxRetries)
						retryNeeded = true
					} else {
						cfg.logf(ctx, "warn", "Patch validation failed after max retries")
						out["patch_apply_error"] = "Patch validation failed: " + validationErrs[0].Error()
					}
				} else {
					// Validation passed, apply patch
					if cfg.Artifacts != nil {
						_ = cfg.Artifacts.SaveArtifact(ctx, cfg.JobID, cfg.Task.ID, cfg.StepID, "patch", p)
					}
					if applyErr := cfg.Patcher.ApplyPatch(ctx, cfg.Task, cfg.Agent, cfg.StepID, p, cfg.WorktreeSuffix); applyErr != nil {
						cfg.logf(ctx, "warn", "failed to apply patch generated by LLM (attempt %d/%d): %v", attempt, maxRetries, applyErr)

						if pErr, ok := applyErr.(*patch.PolicyViolationError); ok {
							if pErr.Severity == patch.SeverityCritical {
								cfg.logf(ctx, "error", "CRITICAL Security Boundary Violation: %s. Pausing task for human review.", pErr.ErrorMsg)
								return nil, false, wrapErr(fmt.Errorf("%w: %s", workflow.ErrPaused, pErr.ErrorMsg))
							}
							if attempt >= 2 {
								cfg.logf(ctx, "error", "Repeated Execution Boundary Violation (attempt %d): %s. Pausing task.", attempt, pErr.ErrorMsg)
								return nil, false, wrapErr(fmt.Errorf("%w: repeated boundary violations: %s", workflow.ErrPaused, pErr.ErrorMsg))
							}

							jsonErrBytes, _ := json.MarshalIndent(pErr, "", "  ")
							retryErrorMsg = fmt.Sprintf("\n\nYour previous patch failed to apply due to an Execution Boundary Violation:\n```json\n%s\n```\nPlease regenerate your patch without modifying files outside the execution boundary.", string(jsonErrBytes))
							retryNeeded = true
							out["patch_apply_error"] = pErr.Error()
							continue
						}

						out["patch_apply_error"] = applyErr.Error()
						updateAffectedFilesWithErrors(ctx, cfg.Task.ID, cfg.Tasks, cfg.Task, applyErr)
						if attempt < maxRetries {
							retryErrorMsg = fmt.Sprintf("\n\nYour previous patch failed to apply with error:\n```text\n%v\n```\nPlease output a corrected patch that applies cleanly.", compressErrorText(applyErr.Error()))
							retryNeeded = true
						} else {
							cfg.logf(ctx, "error", "Patch apply failed after max retries")
							return nil, false, wrapErr(fmt.Errorf("failed to apply patch: %w", applyErr))
						}
					} else {
						// Patch applied cleanly. Now let's run targeted tests to verify!
						hasEditsApplied = true
						if cfg.Tester != nil && cfg.Diff != nil {
							if repoHostPath, err := cfg.Diff.GetTaskRepoHostPath(ctx, cfg.Task); err == nil {
								if changedFiles, diffErr := cfg.Diff.GetChangedFiles(ctx, cfg.Task, cfg.Agent, repoHostPath, cfg.WorktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
									if _, errT := cfg.Tester.RunTargetedTests(ctx, cfg.Task, cfg.Agent, cfg.JobID, cfg.TestLabel, changedFiles, cfg.WorktreeSuffix); errT != nil {
										cfg.logf(ctx, "warn", "targeted tests failed (attempt %d/%d): %v", attempt, maxRetries, errT)
										updateAffectedFilesWithErrors(ctx, cfg.Task.ID, cfg.Tasks, cfg.Task, errT)
										if attempt < maxRetries {
											retryErrorMsg = fmt.Sprintf("\n\nYour patch applied successfully, but the automated tests failed with the following error:\n```text\n%v\n```\nPlease analyze the test failure and output a new patch that fixes this issue.", compressErrorText(errT.Error()))
											retryNeeded = true
										} else {
											cfg.logf(ctx, "error", "Targeted tests failed after max retries")
											return nil, false, wrapErr(fmt.Errorf("targeted tests failed: %w", errT))
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
			} else {
				if attempt < maxRetries {
					cfg.logf(ctx, "warn", "No patch found in response (attempt %d/%d). Retrying...", attempt, maxRetries)
					retryErrorMsg = "\n\nYou failed to provide a diff patch. Please output a valid diff block or SEARCH/REPLACE block as instructed."
					retryNeeded = true
				} else {
					cfg.logf(ctx, "error", "No patch found after max retries")
					return nil, false, wrapErr(fmt.Errorf("LLM failed to provide a valid diff patch"))
				}
			}
		}
		if !retryNeeded {
			break
		}
		if len(cumulativeFailedCalls) > 0 {
			retryErrorMsg += "\n\nNEGATIVE MEMORY (DO NOT REPEAT):\nThe following tool calls failed in previous attempts. Do NOT try to run these exact same tool calls again, as they will fail. Find an alternative path:\n"
			for _, call := range cumulativeFailedCalls {
				retryErrorMsg += "- " + call + "\n"
			}
		}
	}

	return out, patchApplied, nil
}

func compressErrorText(errStr string) string {
	lines := strings.Split(errStr, "\n")
	if len(lines) <= 100 {
		return errStr
	}
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(lines[i] + "\n")
	}
	b.WriteString(fmt.Sprintf("\n... [TRUNCATED: %d lines omitted for brevity] ...\n\n", len(lines)-100))
	for i := len(lines) - 80; i < len(lines); i++ {
		b.WriteString(lines[i] + "\n")
	}
	return strings.TrimSpace(b.String())
}
