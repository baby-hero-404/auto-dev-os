package steps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// BackendAgentAssigner defines the optional hook to assign a role-specific backend agent.
type BackendAgentAssigner interface {
	AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// CodeBackendStep implements Step for the backend coding track.
type CodeBackendStep struct {
	rt          StepRuntime
	tasks       TaskReader
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
	tasks TaskReader,
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
			if err != nil {
				return nil, fmt.Errorf("failed to assign backend agent for backend coding step: %w", err)
			}
			if bg != nil {
				backendAgent = bg
				assignedAgentID = bg.ID
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned backend agent %s for backend coding step", backendAgent.Name))
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
		if s.worktree != nil {
			if targetRepos, err := s.worktree.LoadTargetRepositories(ctx, s.rt.Task); err == nil {
				var ws *models.TaskWorkspace
				if s.workspace != nil {
					ws, _ = s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
				}
				if err := s.worktree.SetupRoleWorktrees(ctx, s.rt.Task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
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

	instruction := "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.\nIMPORTANT: For the patch text, you MUST generate a valid Unified Diff. Ensure that your hunk headers (@@) have the exact correct line counts matching the original file. Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath). DO NOT rewrite the entire file unless creating a new file."
	if isEasy {
		instruction = "Implement the required changes. Return JSON with files_changed, summary, and patch text when available.\nIMPORTANT: For the patch text, you MUST generate a valid Unified Diff. Ensure that your hunk headers (@@) have the exact correct line counts matching the original file. Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath). DO NOT rewrite the entire file unless creating a new file."
	}
	if prFeedback != "" {
		instruction += fmt.Sprintf("\n\nNote: The previous PR was rejected. Address the following PR rejection feedback:\n\n%s\n\n", prFeedback)
	}

	var out map[string]any
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		out, err = s.llm.RunLLMStep(ctx, s.rt.Task, backendAgent, s.rt.JobID, workflow.StepCodeBackend, instruction)
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
						_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepCodeBackend, "patch", p)
					}
					if applyErr := s.patcher.ApplyPatch(ctx, s.rt.Task, backendAgent, workflow.StepCodeBackend, p, worktreeSuffix); applyErr != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to apply patch generated by LLM (attempt %d/%d): %v", attempt, maxRetries, applyErr))
						out["patch_apply_error"] = applyErr.Error()
						if attempt < maxRetries {
							instruction += fmt.Sprintf("\n\nYour previous patch failed to apply with error:\n%v\nPlease output a corrected patch that applies cleanly.", applyErr)
							retryNeeded = true
						} else {
							s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "error", "Patch apply failed after max retries")
							return nil, fmt.Errorf("failed to apply patch: %w", applyErr)
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
		if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, backendAgent, workflow.StepCodeBackend, worktreeSuffix); diffErr == nil && diffText != "" {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepCodeBackend, "diff", diffText)
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

	if worktreeSuffix != "" && s.worktree != nil {
		if targetRepos, err := s.worktree.LoadTargetRepositories(ctx, s.rt.Task); err == nil {
			var ws *models.TaskWorkspace
			if s.workspace != nil {
				ws, _ = s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
			}
			if err := s.worktree.CommitRoleWorktrees(ctx, s.rt.Task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
				return nil, err
			}
		}
	}

	if len(changedFiles) > 0 && s.tester != nil {
		if _, errT := s.tester.RunTargetedTests(ctx, s.rt.Task, backendAgent, s.rt.JobID, "code_backend_test", changedFiles, worktreeSuffix); errT != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
		}
	}
	return out, nil
}
