package steps

import (
	"context"
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
	rt        StepRuntime
	tasks     TaskReader
	llm       LLMRunner
	agents    any
	worktree  WorktreeManager
	patcher   PatchApplier
	diff      DiffCapturer
	workspace WorkspaceLoader
	artifacts ArtifactSaver
	tester    TestRunner
	log       Logger
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
	log Logger,
) *CodeBackendStep {
	return &CodeBackendStep{
		rt:        rt,
		tasks:     tasks,
		llm:       llm,
		agents:    agents,
		worktree:  worktree,
		patcher:   patcher,
		diff:      diff,
		workspace: workspace,
		artifacts: artifacts,
		tester:    tester,
		log:       log,
	}
}

func (s *CodeBackendStep) ID() string                         { return workflow.StepCodeBackend }
func (s *CodeBackendStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CodeBackendStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	backendAgent := s.rt.Agent
	assignedAgentID := ""
	if assigner, ok := s.agents.(BackendAgentAssigner); ok {
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

	worktreeSuffix := ""
	var t *models.Task
	if s.tasks != nil {
		t, _ = s.tasks.GetByID(ctx, s.rt.Task.ID)
	}
	if t != nil && t.Complexity != models.TaskComplexityEasy {
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

	if s.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}

	out, err := s.llm.RunLLMStep(ctx, s.rt.Task, backendAgent, s.rt.JobID, workflow.StepCodeBackend, "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.")
	if err != nil {
		return nil, err
	}
	if parsed, ok := out["parsed"].(map[string]any); ok {
		p := patch.ExtractPatch(parsed)
		if p != "" {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepCodeBackend, "patch", p)
			}
			if s.patcher != nil {
				if applyErr := s.patcher.ApplyPatch(ctx, s.rt.Task, backendAgent, workflow.StepCodeBackend, p, worktreeSuffix); applyErr != nil {
					return nil, fmt.Errorf("apply patch: %w", applyErr)
				}
			}
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
