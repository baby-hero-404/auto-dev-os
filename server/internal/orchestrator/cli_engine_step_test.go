package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeProjectRepo struct {
	project *models.Project
}

func (f *fakeProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return f.project, nil
}

func TestResolveCLIEngineRunner_APINativeReturnsNil(t *testing.T) {
	orch := New(nil, nil, nil, &mockSandboxRuntime{},
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{ExecutionEngine: models.ExecutionEngineAPINative}}),
	)
	task := &models.Task{ProjectID: "proj-1"}
	if r := orch.resolveCLIEngineRunner(context.Background(), task); r != nil {
		t.Fatalf("expected nil runner for api_native engine, got %v", r)
	}
}

func TestResolveCLIEngineRunner_TaskOverride(t *testing.T) {
	cfg := models.CLIEngineConfig{Command: "claude"}
	raw, _ := json.Marshal(cfg)
	orch := New(nil, nil, nil, &mockSandboxRuntime{},
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			ExecutionEngine: models.ExecutionEngineAPINative,
			CLIEngineConfig: raw,
		}}),
	)
	cli := models.ExecutionEngineCLI
	task := &models.Task{ProjectID: "proj-1", ExecutionEngine: &cli}
	r := orch.resolveCLIEngineRunner(context.Background(), task)
	if r == nil {
		t.Fatal("expected a cli engine runner when task overrides to cli")
	}
	if r.cfg.Command != "claude" {
		t.Errorf("expected resolved cli config to carry the project's command, got %q", r.cfg.Command)
	}
}

func TestCLIEngineRunner_RunLLMStep_EndToEnd(t *testing.T) {
	rt := &mockSandboxRuntime{}
	cfg := models.CLIEngineConfig{Command: "claude", Args: []string{"-p", "{prompt_file}"}}
	raw, _ := json.Marshal(cfg)
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			ExecutionEngine: models.ExecutionEngineCLI,
			CLIEngineConfig: raw,
		}}),
		WithWorkspaceRoot(t.TempDir()),
	)

	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	runner := orch.resolveCLIEngineRunner(context.Background(), task)
	if runner == nil {
		t.Fatal("expected a non-nil cli engine runner")
	}

	out, err := runner.RunLLMStep(context.Background(), task, agent, "job-1", "code_backend", "implement the feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, ok := out["parsed"].(map[string]any)
	if !ok {
		t.Fatalf("expected a parsed map in the result, got %#v", out)
	}
	if summary, _ := parsed["summary"].(string); summary == "" {
		t.Errorf("expected a non-empty summary so runPatchRetryLoop treats this as edits-applied")
	}

	// The mock runtime's Run should have been called at least twice: once
	// for the binary-check preflight, once for the real spawn.
	found := false
	for _, cmd := range rt.commands {
		if strings.Contains(cmd, "claude") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected the configured cli command to appear in a sandbox invocation, commands: %v", rt.commands)
	}
}

// noopSandboxRuntime always reports a clean worktree (no changed files),
// simulating a CLI run that completed successfully but touched nothing.
type noopSandboxRuntime struct {
	commands []string
}

func (m *noopSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	return &sandbox.CommandResult{ExitCode: 0, Stdout: ""}, nil
}
func (m *noopSandboxRuntime) Health(ctx context.Context) error  { return nil }
func (m *noopSandboxRuntime) Prewarm(ctx context.Context) error { return nil }

func TestCLIEngineRunner_RunLLMStep_NoChangesFailsByDefault(t *testing.T) {
	rt := &noopSandboxRuntime{}
	cfg := models.CLIEngineConfig{Command: "claude"}
	raw, _ := json.Marshal(cfg)
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			ExecutionEngine: models.ExecutionEngineCLI,
			CLIEngineConfig: raw,
		}}),
		WithWorkspaceRoot(t.TempDir()),
	)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	runner := orch.resolveCLIEngineRunner(context.Background(), task)
	if runner == nil {
		t.Fatal("expected a non-nil cli engine runner")
	}

	_, err := runner.RunLLMStep(context.Background(), task, agent, "job-1", "code_backend", "do nothing")
	if err == nil {
		t.Fatal("expected an error when the cli run produces zero file changes")
	}
	if !strings.Contains(err.Error(), "no file changes") {
		t.Errorf("expected a no-changes error, got: %v", err)
	}
}

func TestCLIEngineRunner_RunLLMStep_NoChangesAllowedWithAllowNoop(t *testing.T) {
	rt := &noopSandboxRuntime{}
	cfg := models.CLIEngineConfig{Command: "claude", AllowNoop: true}
	raw, _ := json.Marshal(cfg)
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			ExecutionEngine: models.ExecutionEngineCLI,
			CLIEngineConfig: raw,
		}}),
		WithWorkspaceRoot(t.TempDir()),
	)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	runner := orch.resolveCLIEngineRunner(context.Background(), task)
	if runner == nil {
		t.Fatal("expected a non-nil cli engine runner")
	}

	if _, err := runner.RunLLMStep(context.Background(), task, agent, "job-1", "code_backend", "inspect only"); err != nil {
		t.Fatalf("expected no error when allow_noop is set, got: %v", err)
	}
}

func TestCLIEngineRunner_RunLLMStep_PreflightRunsOnce(t *testing.T) {
	rt := &mockSandboxRuntime{}
	cfg := models.CLIEngineConfig{Command: "claude"}
	raw, _ := json.Marshal(cfg)
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			ExecutionEngine: models.ExecutionEngineCLI,
			CLIEngineConfig: raw,
		}}),
		WithWorkspaceRoot(t.TempDir()),
	)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	runner := orch.resolveCLIEngineRunner(context.Background(), task)
	if runner == nil {
		t.Fatal("expected a non-nil cli engine runner")
	}

	preflightChecks := func() int {
		n := 0
		for _, cmd := range rt.commands {
			if strings.Contains(cmd, "command -v") {
				n++
			}
		}
		return n
	}

	if _, err := runner.RunLLMStep(context.Background(), task, agent, "job-1", "code_backend", "attempt 1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := runner.RunLLMStep(context.Background(), task, agent, "job-1", "code_backend", "attempt 2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := preflightChecks(); n != 1 {
		t.Errorf("expected exactly 1 preflight binary check across 2 RunLLMStep calls (sync.Once), got %d", n)
	}
}
