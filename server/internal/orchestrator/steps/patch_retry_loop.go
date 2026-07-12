package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
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

	LLM       LLMRunner
	Worktree  WorktreeManager
	Patcher   PatchApplier
	Diff      DiffCapturer
	Artifacts ArtifactSaver
	Tester    TestRunner
	Tasks     TaskRepository
	Log       Logger
}

func (cfg patchRetryConfig) logf(ctx context.Context, level, format string, args ...any) {
	if cfg.Log == nil {
		return
	}
	cfg.Log.Log(ctx, cfg.Task.ID, &cfg.JobID, level, fmt.Sprintf(format, args...))
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
				return nil, false, fmt.Errorf("worktree corrupted: failed to reset worktree: %w", errReset)
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
			return nil, false, err
		}

		retryNeeded := false
		if cfg.Agentic {
			// Edits were already applied via native tool calls inside the agentic loop; the
			// only remaining gate is the same targeted-test verification used by the
			// diff-based path below. A non-empty 'summary' is required as confirmation that
			// the LLM actually completed the work (mirrors llmrunner.Runner's own agentic
			// completion contract), so a vacuous/empty response isn't mistaken for success.
			parsed, _ := out["parsed"].(map[string]any)
			summary, _ := parsed["summary"].(string)
			if strings.TrimSpace(summary) != "" {
				if cfg.Tester != nil && cfg.Diff != nil {
					if repoHostPath, err := cfg.Diff.GetTaskRepoHostPath(ctx, cfg.Task); err == nil {
						if changedFiles, diffErr := cfg.Diff.GetChangedFiles(ctx, cfg.Task, cfg.Agent, repoHostPath, cfg.WorktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
							if _, errT := cfg.Tester.RunTargetedTests(ctx, cfg.Task, cfg.Agent, cfg.JobID, cfg.TestLabel, changedFiles, cfg.WorktreeSuffix); errT != nil {
								cfg.logf(ctx, "warn", "targeted tests failed (attempt %d/%d): %v", attempt, maxRetries, errT)
								updateAffectedFilesWithErrors(ctx, cfg.Task.ID, cfg.Tasks, cfg.Task, errT)
								if attempt < maxRetries {
									retryErrorMsg = fmt.Sprintf("\n\nYour changes applied successfully, but the automated tests failed with the following error:\n%v\nPlease analyze the test failure and use the available tools to fix it.", errT)
									retryNeeded = true
								} else {
									cfg.logf(ctx, "error", "Targeted tests failed after max retries")
									return nil, false, fmt.Errorf("targeted tests failed: %w", errT)
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
			if !retryNeeded {
				break
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
								return nil, false, fmt.Errorf("%w: %s", workflow.ErrPaused, pErr.ErrorMsg)
							}
							if attempt >= 2 {
								cfg.logf(ctx, "error", "Repeated Execution Boundary Violation (attempt %d): %s. Pausing task.", attempt, pErr.ErrorMsg)
								return nil, false, fmt.Errorf("%w: repeated boundary violations: %s", workflow.ErrPaused, pErr.ErrorMsg)
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
							retryErrorMsg = fmt.Sprintf("\n\nYour previous patch failed to apply with error:\n%v\nPlease output a corrected patch that applies cleanly.", applyErr)
							retryNeeded = true
						} else {
							cfg.logf(ctx, "error", "Patch apply failed after max retries")
							return nil, false, fmt.Errorf("failed to apply patch: %w", applyErr)
						}
					} else {
						// Patch applied cleanly. Now let's run targeted tests to verify!
						if cfg.Tester != nil && cfg.Diff != nil {
							if repoHostPath, err := cfg.Diff.GetTaskRepoHostPath(ctx, cfg.Task); err == nil {
								if changedFiles, diffErr := cfg.Diff.GetChangedFiles(ctx, cfg.Task, cfg.Agent, repoHostPath, cfg.WorktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
									if _, errT := cfg.Tester.RunTargetedTests(ctx, cfg.Task, cfg.Agent, cfg.JobID, cfg.TestLabel, changedFiles, cfg.WorktreeSuffix); errT != nil {
										cfg.logf(ctx, "warn", "targeted tests failed (attempt %d/%d): %v", attempt, maxRetries, errT)
										updateAffectedFilesWithErrors(ctx, cfg.Task.ID, cfg.Tasks, cfg.Task, errT)
										if attempt < maxRetries {
											retryErrorMsg = fmt.Sprintf("\n\nYour patch applied successfully, but the automated tests failed with the following error:\n%v\nPlease analyze the test failure and output a new patch that fixes this issue.", errT)
											retryNeeded = true
										} else {
											cfg.logf(ctx, "error", "Targeted tests failed after max retries")
											return nil, false, fmt.Errorf("targeted tests failed: %w", errT)
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
		if !retryNeeded {
			break
		}
	}

	return out, patchApplied, nil
}
