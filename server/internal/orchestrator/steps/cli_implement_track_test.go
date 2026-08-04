package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockTrackAgentAssigner struct {
	backendAgent  *models.Agent
	backendErr    error
	frontendAgent *models.Agent
	frontendErr   error
	released      []string
}

func (m *mockTrackAgentAssigner) AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.backendAgent, m.backendErr
}

func (m *mockTrackAgentAssigner) AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	return m.frontendAgent, m.frontendErr
}

func (m *mockTrackAgentAssigner) Release(ctx context.Context, agentID string) error {
	m.released = append(m.released, agentID)
	return nil
}

func newBackendTrackStep(t *testing.T, root string, rt StepRuntime, agents any, runner CLIStepRunner) *CLIImplementTrackStep {
	t.Helper()
	return NewCLIImplementBackendStep(
		rt,
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIWorktreeManager{},
		&mockWorkspaceLoader{},
		agents,
		runner,
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
		nil,
	)
}

func TestCLIImplementTrackStep_HappyPath_UsesRuntimeAgentWhenRoleMatches(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal")
	writeSpecFiles(t, root, slug, map[string]string{"tasks.md": "- [x] done\n- [ ] todo\n"})

	agent := &models.Agent{ID: "be1", Role: models.AgentRoleBackend}
	runner := &mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"server/main.go"}}}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: agent, JobID: "j1"}, &mockTrackAgentAssigner{}, runner)

	if step.ID() != workflow.StepCLIImplementBackend {
		t.Fatalf("expected step ID %q, got %q", workflow.StepCLIImplementBackend, step.ID())
	}

	res, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["checked_tasks"] != 1 || res["total_tasks"] != 2 {
		t.Errorf("expected 1/2 checkboxes, got %v/%v", res["checked_tasks"], res["total_tasks"])
	}
}

func TestCLIImplementTrackStep_AssignsFreshAgentWhenRoleMismatched(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal")

	runtimeAgent := &models.Agent{ID: "generic1", Role: models.AgentRoleBackend + "-other"}
	backendAgent := &models.Agent{ID: "be2", Role: models.AgentRoleBackend}
	assigner := &mockTrackAgentAssigner{backendAgent: backendAgent}
	runner := &mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"server/main.go"}}}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: runtimeAgent, JobID: "j1"}, assigner, runner)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assigner.released) != 1 || assigner.released[0] != backendAgent.ID {
		t.Errorf("expected freshly-assigned agent %q to be released, got %v", backendAgent.ID, assigner.released)
	}
}

func TestCLIImplementTrackStep_MissingAssignerInterface_Errors(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	runtimeAgent := &models.Agent{ID: "generic1", Role: "other"}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: runtimeAgent, JobID: "j1"}, struct{}{}, &mockCLIStepRunner{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when agents hook does not implement BackendAgentAssigner")
	}
}

func TestCLIImplementTrackStep_AssignerReturnsWrongRole_Errors(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	runtimeAgent := &models.Agent{ID: "generic1", Role: "other"}
	assigner := &mockTrackAgentAssigner{backendAgent: &models.Agent{ID: "fe-oops", Role: models.AgentRoleFrontend}}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: runtimeAgent, JobID: "j1"}, assigner, &mockCLIStepRunner{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when assigned agent's role does not match the track's required role")
	}
}

func TestCLIImplementTrackStep_AssignerError_Propagates(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	runtimeAgent := &models.Agent{ID: "generic1", Role: "other"}
	assigner := &mockTrackAgentAssigner{backendErr: errors.New("no agents available")}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: runtimeAgent, JobID: "j1"}, assigner, &mockCLIStepRunner{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error to propagate from failed agent assignment")
	}
}

func TestCLIImplementTrackStep_NoChangedFiles_Errors(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	agent := &models.Agent{ID: "be1", Role: models.AgentRoleBackend}
	runner := &mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: nil}}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: agent, JobID: "j1"}, &mockTrackAgentAssigner{}, runner)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when no files changed")
	}
}

func TestCLIImplementTrackStep_PausesForClarification(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	agent := &models.Agent{ID: "be1", Role: models.AgentRoleBackend}
	updater := &mockCLITaskUpdater{}
	runner := &mockCLIStepRunner{output: CLIStepOutput{Output: "Which approach?", AwaitingInput: true}}
	step := NewCLIImplementBackendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIWorktreeManager{},
		&mockWorkspaceLoader{},
		&mockTrackAgentAssigner{},
		runner,
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
		updater,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	var pauseErr workflow.PauseError
	if !errors.As(err, &pauseErr) {
		t.Fatalf("expected workflow.PauseError, got: %v", err)
	}
	if pauseErr.Step != workflow.StepCLIImplementBackend {
		t.Errorf("expected pause on %q, got %q", workflow.StepCLIImplementBackend, pauseErr.Step)
	}
}

func TestCLIImplementTrackStep_RunnerError_Propagates(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	agent := &models.Agent{ID: "be1", Role: models.AgentRoleBackend}
	runner := &mockCLIStepRunner{err: errors.New("cli crashed")}
	step := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: agent, JobID: "j1"}, &mockTrackAgentAssigner{}, runner)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error propagated from runner")
	}
}

func TestCLIImplementTrackStep_UsesShortRoleTokenForBranchNaming(t *testing.T) {
	// MergeStep (internal/orchestrator/steps/merge.go) and SetupRoleBranches
	// (internal/orchestrator/repoutil/worktrees.go) both derive role branches
	// with the short "be"/"fe" token (models.RoleBackend/RoleFrontend), not
	// the full agent-role label ("backend"/"frontend"). If this track passed
	// the agent-role label through to SetupRoleWorktrees/CommitRoleWorktrees
	// as the branch-name token, it would commit to a branch MergeStep never
	// looks for and the merge would silently find no work to reconcile.
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal")

	backendAgent := &models.Agent{ID: "be1", Role: models.AgentRoleBackend}
	backendGit := &mockCLIWorktreeManager{}
	backendStep := newBackendTrackStep(t, root, StepRuntime{Task: task, Agent: backendAgent, JobID: "j1"}, &mockTrackAgentAssigner{}, &mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"server/main.go"}}})
	backendStep.git = backendGit

	if _, err := backendStep.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backendGit.gotSetupRoleName != models.RoleBackend {
		t.Errorf("expected SetupRoleWorktrees roleName %q, got %q", models.RoleBackend, backendGit.gotSetupRoleName)
	}
	if backendGit.gotCommitRoleName != models.RoleBackend {
		t.Errorf("expected CommitRoleWorktrees roleName %q, got %q", models.RoleBackend, backendGit.gotCommitRoleName)
	}
	if backendGit.gotSetupRoleLabel != models.AgentRoleBackend {
		t.Errorf("expected SetupRoleWorktrees roleLabel %q, got %q", models.AgentRoleBackend, backendGit.gotSetupRoleLabel)
	}

	frontendAgent := &models.Agent{ID: "fe1", Role: models.AgentRoleFrontend}
	frontendGit := &mockCLIWorktreeManager{}
	frontendStep := NewCLIImplementFrontendStep(
		StepRuntime{Task: task, Agent: frontendAgent, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		frontendGit,
		&mockWorkspaceLoader{},
		&mockTrackAgentAssigner{},
		&mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"web/app.tsx"}}},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
		nil,
	)

	if _, err := frontendStep.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontendGit.gotSetupRoleName != models.RoleFrontend {
		t.Errorf("expected SetupRoleWorktrees roleName %q, got %q", models.RoleFrontend, frontendGit.gotSetupRoleName)
	}
	if frontendGit.gotCommitRoleName != models.RoleFrontend {
		t.Errorf("expected CommitRoleWorktrees roleName %q, got %q", models.RoleFrontend, frontendGit.gotCommitRoleName)
	}
}

func TestCLIImplementTrackStep_UsesWorktreeSuffixForRunnerAndResolver(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal")

	agent := &models.Agent{ID: "fe1", Role: models.AgentRoleFrontend}
	runner := &mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"web/app.tsx"}}}
	resolver := &mockWorktreeHostPathResolver{root: root}
	step := NewCLIImplementFrontendStep(
		StepRuntime{Task: task, Agent: agent, JobID: "j1"},
		resolver,
		&mockCLIWorktreeManager{},
		&mockWorkspaceLoader{},
		&mockTrackAgentAssigner{},
		runner,
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
		nil,
	)

	if step.ID() != workflow.StepCLIImplementFrontend {
		t.Fatalf("expected step ID %q, got %q", workflow.StepCLIImplementFrontend, step.ID())
	}

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.gotWorktreeSuffix != models.WorktreeSuffixFrontend {
		t.Errorf("expected runner to receive worktree suffix %q, got %q", models.WorktreeSuffixFrontend, runner.gotWorktreeSuffix)
	}
}
