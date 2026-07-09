package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// BackendAgentAssigner defines the optional hook to assign a role-specific backend agent.
type BackendAgentAssigner interface {
	AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// CodeBackendStep implements Step for the backend coding track.
type CodeBackendStep struct {
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

func NewCodeBackendStep(
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
) *CodeBackendStep {
	return &CodeBackendStep{
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

func (s *CodeBackendStep) ID() string                         { return workflow.StepCodeBackend }
func (s *CodeBackendStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CodeBackendStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	var t *models.Task
	if s.tasks != nil {
		t, _ = s.tasks.GetByID(ctx, s.rt.Task.ID)
	}
	isEasy := t != nil && t.Complexity == models.TaskComplexityEasy

	backendAgent := s.rt.Agent
	assignedAgentID := ""

	// Finding 6: Role-Specialization Bypassed in EasyWorkflow
	if !isEasy {
		if backendAgent == nil || backendAgent.Role != models.AgentRoleBackend {
			assigner, ok := s.agents.(BackendAgentAssigner)
			if !ok {
				roleStr := "nil"
				if backendAgent != nil {
					roleStr = backendAgent.Role
				}
				return nil, fmt.Errorf("backend coding step requires a backend agent, but got role %s", roleStr)
			}
			bg, err := assigner.AssignBackendAgent(ctx, s.rt.Task)
			if bg != nil {
				backendAgent = bg
				assignedAgentID = bg.ID
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned backend agent %s for backend coding step", backendAgent.Name))
			}
			if err != nil {
				if assignedAgentID != "" {
					if releaser, ok := s.agents.(AgentReleaser); ok {
						_ = releaser.Release(context.WithoutCancel(ctx), assignedAgentID)
					}
				}
				return nil, fmt.Errorf("failed to assign backend agent for backend coding step: %w", err)
			}
		}
		if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
			defer func() {
				if releaser, ok := s.agents.(AgentReleaser); ok {
					if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release backend agent failed: %v", err))
					}
				}
			}()
		}
		if backendAgent == nil || backendAgent.Role != models.AgentRoleBackend {
			roleStr := "nil"
			if backendAgent != nil {
				roleStr = backendAgent.Role
			}
			return nil, fmt.Errorf("backend coding step requires a backend agent, but got role %s", roleStr)
		}
	} else if backendAgent == nil {
		return nil, fmt.Errorf("coding step requires an agent")
	}

	worktreeSuffix := ""
	if !isEasy {
		worktreeSuffix = "-be-worktree"
		if err := setupSandbox(ctx, s.rt.Task, backendAgent, s.worktree, s.workspace, "be", "backend", worktreeSuffix); err != nil {
			return nil, err
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

	instructionBase := "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.\nIMPORTANT: For the patch text, you MUST generate a valid Unified Diff. Ensure that your hunk headers (@@) have the exact correct line counts matching the original file."
	var pathCtx *paths.AgentPathContext
	repoContext := ""
	var physicalRoot string
	if s.workspace != nil {
		if ws, _ := s.workspace.LoadTaskWorkspace(ctx, s.rt.Task); ws != nil {
			var useRepoPrefix bool
			var repoName string
			role := "backend"

			if len(ws.Repos) == 1 {
				repoName = ws.Repos[0].Name
				useRepoPrefix = false
				if isEasy {
					physicalRoot = paths.NewOSWorkspacePaths(filepath.Dir(ws.Root)).RepoMain(s.rt.Task.ID, repoName).String()
				} else {
					physicalRoot = paths.NewOSWorkspacePaths(filepath.Dir(ws.Root)).RepoWorktreeDir(s.rt.Task.ID, repoName, role).String()
				}
				repoContext = "\nIMPORTANT: Your workspace root IS the repository root.\nAll file paths MUST be relative (e.g., internal/model/commit.go).\nDo NOT prefix with the repository name.\nYour diff paths MUST be relative to the repository root, e.g., --- a/filepath. DO NOT include the repository name in the path."
			} else {
				useRepoPrefix = true
				physicalRoot = paths.NewOSWorkspacePaths(filepath.Dir(ws.Root)).CodeRoot(s.rt.Task.ID).String()
				var names []string
				for _, r := range ws.Repos {
					names = append(names, r.Name)
				}
				repoContext = fmt.Sprintf(" You are working on multiple repositories: %s. Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath).", strings.Join(names, ", "))
			}
			pathCtx = paths.NewAgentPathContext(physicalRoot, useRepoPrefix, repoName, role)
			ctx = context.WithValue(ctx, paths.AgentPathContextKey, pathCtx)
		}
	}
	if repoContext == "" {
		repoContext = " Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath)."
	}
	instruction := instructionBase + repoContext + " DO NOT rewrite the entire file unless creating a new file."
	if prFeedback != "" {
		instruction += fmt.Sprintf("\n\nNote: The previous PR was rejected. Address the following PR rejection feedback:\n\n%s\n\n", prFeedback)
	}

	// Perform repository structure scan (Task 2.1) (REQ-M02)
	var tree string
	if contextLoadOut, ok := stepCtx.Inputs[workflow.StepContextLoad]; ok {
		if cacheJSON, ok := contextLoadOut["context_cache"].(string); ok && cacheJSON != "" {
			var cache models.ContextCache
			if err := json.Unmarshal([]byte(cacheJSON), &cache); err == nil && cache.DirectoryTree != "" {
				tree = cache.DirectoryTree
			}
		}
	}
	if tree == "" && physicalRoot != "" {
		if t, err := ScanDirectory(physicalRoot, 3, 200); err == nil && t != "" {
			tree = t
		}
	}
	if tree != "" {
		instruction += fmt.Sprintf("\n\n=== Repository Structure ===\n%s\n", tree)
	}
	// Inject role-specific subtasks from Plan output
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if beTasks, ok := subtasks["backend"].([]any); ok && len(beTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}

				if taskIdx >= 0 && taskIdx < len(beTasks) {
					instruction += fmt.Sprintf("\n\n## Your Assigned Subtask:\n%s\n", beTasks[taskIdx])
				} else {
					instruction += "\n\n## Your Assigned Subtasks:\n"
					for i, t := range beTasks {
						instruction += fmt.Sprintf("%d. %s\n", i+1, t)
					}
				}
			}
		}
	}

	// Inject files created/modified by prior steps (Task 2.4)
	var priorFiles []string
	seenPriorFiles := make(map[string]bool)
	for inputStepID, stepOut := range stepCtx.Inputs {
		if strings.HasPrefix(inputStepID, workflow.StepCodeBackend) || strings.HasPrefix(inputStepID, workflow.StepCodeFrontend) {
			if fc, ok := stepOut["files_changed"]; ok {
				var filesList []string
				if fl, ok := fc.([]any); ok {
					for _, f := range fl {
						if str, ok := f.(string); ok {
							filesList = append(filesList, str)
						}
					}
				} else if fl, ok := fc.([]string); ok {
					filesList = fl
				}
				for _, f := range filesList {
					if !seenPriorFiles[f] {
						seenPriorFiles[f] = true
						priorFiles = append(priorFiles, f)
					}
				}
			}
		}
	}
	if len(priorFiles) > 0 {
		instruction += "\n\n### Files Created/Modified by Prior Steps ###\n"
		for _, f := range priorFiles {
			instruction += fmt.Sprintf("- %s\n", f)
		}
	}

	var out map[string]any
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		out, err = s.llm.RunLLMStep(ctx, s.rt.Task, backendAgent, s.rt.JobID, stepCtx.StepID, instruction)
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
					if applyErr := s.patcher.ApplyPatch(ctx, s.rt.Task, backendAgent, stepCtx.StepID, p, worktreeSuffix); applyErr != nil {
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
								if changedFiles, diffErr := s.diff.GetChangedFiles(ctx, s.rt.Task, backendAgent, repoHostPath, worktreeSuffix); diffErr == nil && len(changedFiles) > 0 {
									if _, errT := s.tester.RunTargetedTests(ctx, s.rt.Task, backendAgent, s.rt.JobID, "code_backend_test", changedFiles, worktreeSuffix); errT != nil {
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
		if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, backendAgent, stepCtx.StepID, worktreeSuffix); diffErr == nil && diffText != "" {
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
			changedFiles, diffErr = s.diff.GetChangedFiles(ctx, s.rt.Task, backendAgent, repoHostPath, worktreeSuffix)
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

	if worktreeSuffix != "" {
		if err := commitSandbox(ctx, s.rt.Task, backendAgent, s.worktree, s.workspace, "be", "backend", worktreeSuffix); err != nil {
			return nil, err
		}
	}

	// Mark subtask as complete in the database if applicable
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if beTasks, ok := subtasks["backend"].([]any); ok && len(beTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}
				if taskIdx >= 0 && taskIdx < len(beTasks) {
					taskText := beTasks[taskIdx].(string)

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

	if out == nil {
		out = make(map[string]any)
	}
	out["files_changed"] = changedFiles

	return out, nil
}
