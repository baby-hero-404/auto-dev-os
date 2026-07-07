package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// FrontendAgentAssigner defines the optional hook to assign a role-specific frontend agent.
type FrontendAgentAssigner interface {
	AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// CodeFrontendStep implements Step for the frontend coding track.
type CodeFrontendStep struct {
	rt          StepRuntime
	tasks       TaskRepository
	llm         LLMRunner
	agents      any
	worktree    WorktreeManager
	patcher     PatchApplier
	diff        DiffCapturer
	workspace   WorkspaceLoader
	artifacts   ArtifactSaver
	tester      TestRunner
	checkpoints CheckpointLister
	log         Logger
}

func NewCodeFrontendStep(
	rt StepRuntime,
	tasks TaskRepository,
	llm LLMRunner,
	agents any,
	worktree WorktreeManager,
	patcher PatchApplier,
	diff DiffCapturer,
	workspace WorkspaceLoader,
	artifacts ArtifactSaver,
	tester TestRunner,
	checkpoints CheckpointLister,
	log Logger,
) *CodeFrontendStep {
	return &CodeFrontendStep{
		rt:          rt,
		tasks:       tasks,
		llm:         llm,
		agents:      agents,
		worktree:    worktree,
		patcher:     patcher,
		diff:        diff,
		workspace:   workspace,
		artifacts:   artifacts,
		tester:      tester,
		checkpoints: checkpoints,
		log:         log,
	}
}

func (s *CodeFrontendStep) ID() string                         { return workflow.StepCodeFrontend }
func (s *CodeFrontendStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CodeFrontendStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	var t *models.Task
	var err error
	if s.tasks != nil {
		t, err = s.tasks.GetByID(ctx, s.rt.Task.ID)
	}

	hasFrontendFiles := false
	if err == nil && t != nil {
		if t.Complexity == models.TaskComplexityEasy {
			return StepResult{"status": "skipped", "info": "skipped frontend step for easy task"}, nil
		}
		var analysis models.TaskAnalysis
		if json.Unmarshal(t.Analysis, &analysis) == nil {
			for _, file := range analysis.AffectedFiles {
				if isFrontendFile(file.File) {
					hasFrontendFiles = true
					break
				}
			}
		}
	}

	// Check skip signal from Plan step (Phase 3)
	if !hasFrontendFiles {
		if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
			if skip, _ := planOut["skip_frontend"].(bool); skip {
				return StepResult{"status": "skipped", "info": "skipped by plan: no frontend work needed"}, nil
			}
		} else if err == nil && t != nil {
			// Fallback if Plan inputs are missing (e.g. in tests)
			return StepResult{"status": "skipped", "info": "no frontend files affected"}, nil
		}
	}

	frontendAgent := s.rt.Agent
	assignedAgentID := ""
	if frontendAgent == nil || frontendAgent.Role != models.AgentRoleFrontend {
		assigner, ok := s.agents.(FrontendAgentAssigner)
		if !ok {
			roleStr := "nil"
			if frontendAgent != nil {
				roleStr = frontendAgent.Role
			}
			return nil, fmt.Errorf("frontend coding step requires a frontend agent, but got role %s", roleStr)
		}
		fg, err := assigner.AssignFrontendAgent(ctx, s.rt.Task)
		if err != nil {
			return nil, fmt.Errorf("failed to assign frontend agent for frontend coding step: %w", err)
		}
		if fg != nil {
			frontendAgent = fg
			assignedAgentID = fg.ID
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned frontend agent %s for frontend coding step", frontendAgent.Name))
		}
	}
	if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
		defer func() {
			if releaser, ok := s.agents.(AgentReleaser); ok {
				if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
					s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release frontend agent failed: %v", err))
				}
			}
		}()
	}
	if frontendAgent == nil || frontendAgent.Role != models.AgentRoleFrontend {
		roleStr := "nil"
		if frontendAgent != nil {
			roleStr = frontendAgent.Role
		}
		return nil, fmt.Errorf("frontend coding step requires a frontend agent, but got role %s", roleStr)
	}

	worktreeSuffix := ""
	if t != nil && t.Complexity != models.TaskComplexityEasy {
		worktreeSuffix = "-fe-worktree"
		if s.worktree != nil {
			if targetRepos, err := s.worktree.LoadTargetRepositories(ctx, s.rt.Task); err == nil {
				var ws *models.TaskWorkspace
				if s.workspace != nil {
					ws, _ = s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
				}
				if err := s.worktree.SetupRoleWorktrees(ctx, s.rt.Task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}
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

	instructionBase := "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available.\nIMPORTANT: For the patch text, you MUST generate a valid Unified Diff. Ensure that your hunk headers (@@) have the exact correct line counts matching the original file."
	
	repoContext := ""
	if s.workspace != nil {
		if ws, _ := s.workspace.LoadTaskWorkspace(ctx, s.rt.Task); ws != nil {
			if len(ws.Repos) == 1 {
				repoContext = " Your diff paths MUST be relative to the repository root, e.g., --- a/filepath. DO NOT include the repository name in the path."
			} else if len(ws.Repos) > 1 {
				var names []string
				for _, r := range ws.Repos {
					names = append(names, r.Name)
				}
				repoContext = fmt.Sprintf(" You are working on multiple repositories: %s. Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath).", strings.Join(names, ", "))
			}
		}
	}
	if repoContext == "" {
		repoContext = " Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath)."
	}
	instruction := instructionBase + repoContext + " DO NOT rewrite the entire file unless creating a new file."
	if prFeedback != "" {
		instruction += fmt.Sprintf("\n\nNote: The previous PR was rejected. Address the following PR rejection feedback:\n\n%s\n\n", prFeedback)
	}
	// Inject role-specific subtasks from Plan output
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if feTasks, ok := subtasks["frontend"].([]any); ok && len(feTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}

				if taskIdx >= 0 && taskIdx < len(feTasks) {
					instruction += fmt.Sprintf("\n\n## Your Assigned Subtask:\n%s\n", feTasks[taskIdx])
				} else {
					instruction += "\n\n## Your Assigned Subtasks:\n"
					for i, t := range feTasks {
						instruction += fmt.Sprintf("%d. %s\n", i+1, t)
					}
				}
			}
		}
	}

	var out map[string]any
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		out, err = s.llm.RunLLMStep(ctx, s.rt.Task, frontendAgent, s.rt.JobID, stepCtx.StepID, instruction)
		if err != nil {
			return nil, err
		}

		retryNeeded := false
		if parsed, ok := out["parsed"].(map[string]any); ok {
			p := patch.ExtractPatch(parsed)
			if p != "" && s.patcher != nil {
				// Validate
				validationErrs := s.patcher.Validate(ctx, s.rt.Task, p, worktreeSuffix)
				if len(validationErrs) > 0 {
					if attempt < maxRetries {
						errMsg := ""
						for _, ve := range validationErrs {
							errMsg += "- " + ve.Error() + "\n"
						}
						instruction += fmt.Sprintf("\n\nYour previous patch failed validation. Please fix the following errors:\n%s\nOutput the patch again.", errMsg)
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("Patch validation failed (attempt %d/%d). Retrying...", attempt, maxRetries))
						retryNeeded = true
					} else {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "Patch validation failed after max retries")
						out["patch_apply_error"] = "Patch validation failed: " + validationErrs[0].Error()
					}
				} else {
					// Validation passed, apply patch
					if s.artifacts != nil {
						_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, stepCtx.StepID, "patch", p)
					}
					if applyErr := s.patcher.ApplyPatch(ctx, s.rt.Task, frontendAgent, stepCtx.StepID, p, worktreeSuffix); applyErr != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to apply patch generated by LLM (attempt %d/%d): %v", attempt, maxRetries, applyErr))
						
						if pErr, ok := applyErr.(*patch.PolicyViolationError); ok {
							if pErr.Severity == patch.SeverityCritical {
								s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", fmt.Sprintf("CRITICAL Security Boundary Violation: %s. Pausing task for human review.", pErr.ErrorMsg))
								return nil, fmt.Errorf("%w: %s", workflow.ErrPaused, pErr.ErrorMsg)
							}
							if attempt >= 2 {
								s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", fmt.Sprintf("Repeated Execution Boundary Violation (attempt %d): %s. Pausing task.", attempt, pErr.ErrorMsg))
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
							s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", "Patch apply failed after max retries")
							return nil, fmt.Errorf("failed to apply patch: %w", applyErr)
						}
					} else {
						// Patch applied cleanly. Now let's run targeted tests to verify!
						if s.tester != nil && s.diff != nil {
							if repoHostPath, err := s.diff.GetTaskRepoHostPath(ctx, s.rt.Task); err == nil {
								if changedFiles, diffErr := s.diff.GetChangedFiles(ctx, s.rt.Task, frontendAgent, repoHostPath, worktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
									if _, errT := s.tester.RunTargetedTests(ctx, s.rt.Task, frontendAgent, s.rt.JobID, "code_frontend_test", changedFiles, worktreeSuffix); errT != nil {
										s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("targeted tests failed (attempt %d/%d): %v", attempt, maxRetries, errT))
										if attempt < maxRetries {
											instruction += fmt.Sprintf("\n\nYour patch applied successfully, but the automated tests failed with the following error:\n%v\nPlease analyze the test failure and output a new patch that fixes this issue.", errT)
											retryNeeded = true
										} else {
											s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", "Targeted tests failed after max retries")
											return nil, fmt.Errorf("targeted tests failed: %w", errT)
										}
									}
								}
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
		if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, frontendAgent, stepCtx.StepID, worktreeSuffix); diffErr == nil && diffText != "" {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, stepCtx.StepID, "diff", diffText)
			}
		}
	}

	var changedFiles []string
	if s.diff != nil {
		repoHostPath, err := s.diff.GetTaskRepoHostPath(ctx, s.rt.Task)
		if err == nil {
			var diffErr error
			changedFiles, diffErr = s.diff.GetChangedFiles(ctx, s.rt.Task, frontendAgent, repoHostPath, worktreeSuffix)
			if diffErr != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
			}
		}
	}

	if len(changedFiles) > 0 {
		err := updateTaskAnalysis(ctx, s.rt.Task.ID, s.tasks, s.rt.Task, func(analysis *models.TaskAnalysis) bool {
			updated := false
			for _, newFile := range changedFiles {
				exists := false
				for _, f := range analysis.AffectedFiles {
					if f.File == newFile {
						exists = true
						break
					}
				}
				if !exists {
					analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{File: newFile})
					updated = true
				}
			}
			return updated
		})
		if err != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to update analysis affected files: %v", err))
		}
	}

	if worktreeSuffix != "" && s.worktree != nil {
		if targetRepos, err := s.worktree.LoadTargetRepositories(ctx, s.rt.Task); err == nil {
			var ws *models.TaskWorkspace
			if s.workspace != nil {
				ws, _ = s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
			}
			if err := s.worktree.CommitRoleWorktrees(ctx, s.rt.Task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
				return nil, err
			}
		}
	}

	// Mark subtask as complete in the database if applicable
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if feTasks, ok := subtasks["frontend"].([]any); ok && len(feTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}
				if taskIdx >= 0 && taskIdx < len(feTasks) {
					taskText := feTasks[taskIdx].(string)

					err := updateTaskAnalysis(ctx, s.rt.Task.ID, s.tasks, s.rt.Task, func(analysis *models.TaskAnalysis) bool {
						if updatedTasksMD, ok := updateTaskSubtaskMarkdown(analysis.TasksMD, taskText); ok {
							analysis.TasksMD = updatedTasksMD
							return true
						}
						return false
					})
					if err != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to update tasks_md status: %v", err))
					}
				}
			}
		}
	}

	return out, nil
}
