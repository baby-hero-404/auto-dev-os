package steps

import (
	"context"
	"encoding/json"
	"fmt"
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

func NewCodeFrontendStep(
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
	if err == nil && t != nil {
		if t.Complexity == models.TaskComplexityEasy {
			return StepResult{"status": "skipped", "info": "skipped frontend step for easy task"}, nil
		}
		var analysis models.TaskAnalysis
		if json.Unmarshal(t.Analysis, &analysis) == nil {
			hasFrontend := false
			for _, file := range analysis.AffectedFiles {
				if strings.HasPrefix(file, "web/") || strings.HasSuffix(file, ".tsx") || strings.HasSuffix(file, ".css") || strings.HasSuffix(file, ".html") {
					hasFrontend = true
					break
				}
			}
			if !hasFrontend {
				return StepResult{"status": "skipped", "info": "no frontend files affected"}, nil
			}
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

	instruction := "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available."
	if prFeedback != "" {
		instruction += fmt.Sprintf("\n\nNote: The previous PR was rejected. Address the following PR rejection feedback:\n\n%s\n\n", prFeedback)
	}

	out, err := s.llm.RunLLMStep(ctx, s.rt.Task, frontendAgent, s.rt.JobID, workflow.StepCodeFrontend, instruction)
	if err != nil {
		return nil, err
	}
	if parsed, ok := out["parsed"].(map[string]any); ok {
		p := patch.ExtractPatch(parsed)
		if p != "" {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepCodeFrontend, "patch", p)
			}
			if s.patcher != nil {
				if applyErr := s.patcher.ApplyPatch(ctx, s.rt.Task, frontendAgent, workflow.StepCodeFrontend, p, worktreeSuffix); applyErr != nil {
					return nil, fmt.Errorf("apply patch: %w", applyErr)
				}
			}
		}
	}
	if s.diff != nil {
		if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, frontendAgent, workflow.StepCodeFrontend, worktreeSuffix); diffErr == nil && diffText != "" {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepCodeFrontend, "diff", diffText)
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

	if len(changedFiles) > 0 && s.tester != nil {
		if _, errT := s.tester.RunTargetedTests(ctx, s.rt.Task, frontendAgent, s.rt.JobID, "code_frontend_test", changedFiles, worktreeSuffix); errT != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
		}
	}
	return out, nil
}
