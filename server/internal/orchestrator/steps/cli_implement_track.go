package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// CLIImplementTrackStep implements Step for one parallel track (backend or
// frontend) of the CLI dual-agent implement stage (Phase 4, "Two CLI
// agents" decision). It mirrors CLIImplementStep's instruction-building and
// output-validation logic, but assigns a role-specific agent
// (BackendAgentAssigner/FrontendAgentAssigner, same interfaces
// code_backend.go/code_frontend.go use) and runs the CLI subprocess inside
// its own git worktree (setupSandbox/commitSandbox), so the two tracks'
// concurrent runs never clobber the same checkout. StepMerge then
// reconciles both role branches, exactly as it already does for the
// API-native code_backend/code_frontend split.
type CLIImplementTrackStep struct {
	rt         StepRuntime
	role       string // models.AgentRoleBackend or models.AgentRoleFrontend (agent-role matching/assignment)
	branchRole string // models.RoleBackend ("be") or models.RoleFrontend ("fe") — branch-name/step-id
	// suffix token. Must match what MergeStep (internal/orchestrator/steps/merge.go)
	// and SetupRoleBranches (internal/orchestrator/repoutil/worktrees.go) derive
	// role branches with — NOT the agent-role label, which uses the full
	// "backend"/"frontend" spelling and would otherwise leave this track
	// committing to a branch MergeStep never looks for.
	stepID         string // workflow.StepCLIImplementBackend/Frontend
	worktreeSuffix string // models.WorktreeSuffixBackend/Frontend
	worktree       WorktreeHostPathResolver
	git            WorktreeManager
	workspace      WorkspaceLoader
	agents         any
	runner         CLIStepRunner
	prompts        StepPromptLoader
	log            Logger
	checkpoints    CheckpointLister
	tasks          TaskUpdater
}

func newCLIImplementTrackStep(
	rt StepRuntime,
	role, branchRole, stepID, worktreeSuffix string,
	worktree WorktreeHostPathResolver,
	git WorktreeManager,
	workspace WorkspaceLoader,
	agents any,
	runner CLIStepRunner,
	prompts StepPromptLoader,
	log Logger,
	checkpoints CheckpointLister,
	tasks TaskUpdater,
) *CLIImplementTrackStep {
	return &CLIImplementTrackStep{
		rt:             rt,
		role:           role,
		branchRole:     branchRole,
		stepID:         stepID,
		worktreeSuffix: worktreeSuffix,
		worktree:       worktree,
		git:            git,
		workspace:      workspace,
		agents:         agents,
		runner:         runner,
		prompts:        prompts,
		log:            log,
		checkpoints:    checkpoints,
		tasks:          tasks,
	}
}

// NewCLIImplementBackendStep constructs the backend track of the parallel
// CLI implement stage.
func NewCLIImplementBackendStep(
	rt StepRuntime, worktree WorktreeHostPathResolver, git WorktreeManager, workspace WorkspaceLoader,
	agents any, runner CLIStepRunner, prompts StepPromptLoader, log Logger, checkpoints CheckpointLister, tasks TaskUpdater,
) *CLIImplementTrackStep {
	return newCLIImplementTrackStep(rt, models.AgentRoleBackend, models.RoleBackend, workflow.StepCLIImplementBackend, models.WorktreeSuffixBackend,
		worktree, git, workspace, agents, runner, prompts, log, checkpoints, tasks)
}

// NewCLIImplementFrontendStep constructs the frontend track of the parallel
// CLI implement stage.
func NewCLIImplementFrontendStep(
	rt StepRuntime, worktree WorktreeHostPathResolver, git WorktreeManager, workspace WorkspaceLoader,
	agents any, runner CLIStepRunner, prompts StepPromptLoader, log Logger, checkpoints CheckpointLister, tasks TaskUpdater,
) *CLIImplementTrackStep {
	return newCLIImplementTrackStep(rt, models.AgentRoleFrontend, models.RoleFrontend, workflow.StepCLIImplementFrontend, models.WorktreeSuffixFrontend,
		worktree, git, workspace, agents, runner, prompts, log, checkpoints, tasks)
}

func (s *CLIImplementTrackStep) ID() string { return s.stepID }

func (s *CLIImplementTrackStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CLIImplementTrackStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	agent, assignedAgentID, err := s.assignRoleAgent(ctx)
	if err != nil {
		return nil, err
	}
	if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
		defer func() {
			if releaser, ok := s.agents.(AgentReleaser); ok {
				if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
					s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release %s agent failed: %v", s.role, err))
				}
			}
		}()
	}

	if err := setupSandbox(ctx, s.rt.Task, agent, s.git, s.workspace, s.branchRole, s.role, s.worktreeSuffix); err != nil {
		return nil, fmt.Errorf("%s: setup worktree: %w", s.ID(), err)
	}

	base, err := s.prompts.LoadStepPrompt(workflow.StepCLIImplement)
	if err != nil {
		return nil, fmt.Errorf("%s: load prompt: %w", s.ID(), err)
	}

	rolePrompt, err := s.prompts.LoadRolePrompt(agent, *s.rt.Task, s.ID())
	if err != nil {
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to load role prompt: %v", err))
	}
	if rolePrompt != "" {
		base = rolePrompt + "\n\n" + base
	}

	slug := TaskSpecSlug(s.rt.Task)
	specDir := fmt.Sprintf("docs/openspecs/%s", slug)
	instruction := fmt.Sprintf(
		"%s\n\n## Task\n\n### %s\n\n%s\n\n## Spec set location\n\n%s/\n\n## Your track\n\nYou are the %s implementer, running in an isolated git worktree. Only make %s changes — another agent is implementing the other side of this task concurrently in a separate worktree; a merge step will reconcile both afterward.\n",
		base, s.rt.Task.Title, s.rt.Task.Description, specDir, s.role, s.role,
	)
	if feedback := crossReviewFeedback(ctx, s.checkpoints, s.rt.Task.ID); feedback != "" {
		instruction += "\n\n## Reviewer feedback\n\n" + feedback
	}
	instruction += "\n" + cliWorkingDirNotice + "\n"

	contextFiles, err := s.prompts.MaterializeCLIContext(ctx, *s.rt.Task, agent, s.ID())
	if err != nil {
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to materialize context: %v", err))
	}
	if len(contextFiles) > 0 {
		instruction += "\n" + cliPlatformContextPointer + "\n"
	}

	out, err := s.runner.RunCLIStep(ctx, s.rt.Task, agent, s.rt.JobID, s.ID(), instruction, nil, contextFiles, s.worktreeSuffix)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.ID(), err)
	}
	if out.AwaitingInput {
		return pauseForClarification(ctx, s.tasks, s.rt.Task, s.ID(), out.Output)
	}

	if len(out.ChangedFiles) == 0 {
		return nil, fmt.Errorf("%s: run completed but produced no file changes", s.ID())
	}

	root, err := s.worktree.ResolveHostWorktreeRoot(ctx, s.rt.Task, s.worktreeSuffix)
	if err != nil {
		return nil, fmt.Errorf("%s: resolve worktree: %w", s.ID(), err)
	}

	docsOnly := isDocsOnlyTask(s.rt.Task) || proposalDeclaresDocumentationOnly(root, slug)
	if !docsOnly && !hasChangeOutsideOpenspecs(out.ChangedFiles) {
		return nil, fmt.Errorf("%s: implement produced no code changes (only docs/openspecs/ was touched)", s.ID())
	}

	if s.git != nil {
		if _, err := s.git.CreateGitCheckpoint(ctx, s.rt.Task, agent, s.ID(), s.worktreeSuffix); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("%s: failed to create git checkpoint: %v", s.ID(), err))
		}
	}

	if err := commitSandbox(ctx, s.rt.Task, agent, s.git, s.workspace, s.branchRole, s.role, s.worktreeSuffix); err != nil {
		return nil, fmt.Errorf("%s: commit worktree: %w", s.ID(), err)
	}

	tasksMD, _ := os.ReadFile(filepath.Join(root, "docs", "openspecs", slug, "tasks.md"))
	done, total := workflow.ParseCheckboxes(string(tasksMD))

	return StepResult{"status": "success", "checked_tasks": done, "total_tasks": total}, nil
}

// assignRoleAgent resolves the agent to use for this track: the runtime
// agent if it already matches the required role, otherwise a fresh
// assignment via BackendAgentAssigner/FrontendAgentAssigner (same optional
// hooks code_backend.go/code_frontend.go use), mirroring their error
// handling exactly.
func (s *CLIImplementTrackStep) assignRoleAgent(ctx context.Context) (*models.Agent, string, error) {
	agent := s.rt.Agent
	if agent != nil && agent.Role == s.role {
		return agent, "", nil
	}

	switch s.role {
	case models.AgentRoleBackend:
		assigner, ok := s.agents.(BackendAgentAssigner)
		if !ok {
			return nil, "", fmt.Errorf("%s: requires a backend agent, but got role %s", s.ID(), agentRoleOrNil(agent))
		}
		ag, err := assigner.AssignBackendAgent(ctx, s.rt.Task)
		return s.finishAssign(ctx, ag, err)
	case models.AgentRoleFrontend:
		assigner, ok := s.agents.(FrontendAgentAssigner)
		if !ok {
			return nil, "", fmt.Errorf("%s: requires a frontend agent, but got role %s", s.ID(), agentRoleOrNil(agent))
		}
		ag, err := assigner.AssignFrontendAgent(ctx, s.rt.Task)
		return s.finishAssign(ctx, ag, err)
	default:
		return nil, "", fmt.Errorf("%s: unsupported role %q", s.ID(), s.role)
	}
}

func (s *CLIImplementTrackStep) finishAssign(ctx context.Context, agent *models.Agent, err error) (*models.Agent, string, error) {
	if agent == nil {
		if err != nil {
			return nil, "", fmt.Errorf("%s: failed to assign %s agent: %w", s.ID(), s.role, err)
		}
		return nil, "", fmt.Errorf("%s: requires a %s agent, but got role nil", s.ID(), s.role)
	}
	s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned %s agent %s for %s", s.role, agent.Name, s.ID()))
	if err != nil {
		if releaser, ok := s.agents.(AgentReleaser); ok {
			_ = releaser.Release(context.WithoutCancel(ctx), agent.ID)
		}
		return nil, "", fmt.Errorf("%s: failed to assign %s agent: %w", s.ID(), s.role, err)
	}
	if agent.Role != s.role {
		return nil, "", fmt.Errorf("%s: requires a %s agent, but got role %s", s.ID(), s.role, agent.Role)
	}
	return agent, agent.ID, nil
}

func agentRoleOrNil(agent *models.Agent) string {
	if agent == nil {
		return "nil"
	}
	return agent.Role
}
