package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestCLIStepRunner_ResolveConfig_ExecutionProviders(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	runner := newCLIStepRunner(orch)
	cfg, orgID, err := runner.resolveConfig(context.Background(), &models.Task{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Command != "claude" {
		t.Errorf("expected config built from CLIProfiles[claude_code], got command %q", cfg.Command)
	}
	if orgID != "org1" {
		t.Errorf("expected orgID org1, got %q", orgID)
	}
	if runner.credID != "cred-claude" {
		t.Errorf("expected credID threaded through for cooldown write-back, got %q", runner.credID)
	}
}

func TestCLIStepRunner_ResolveConfig_LegacyFallback(t *testing.T) {
	cfg := models.CLIEngineConfig{Command: "claude", Args: []string{"-p"}}
	raw, _ := json.Marshal(cfg)
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			ExecutionEngine: models.ExecutionEngineCLI,
			CLIEngineConfig: raw,
		}}),
	)
	runner := newCLIStepRunner(orch)
	resolved, _, err := runner.resolveConfig(context.Background(), &models.Task{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Command != "claude" {
		t.Errorf("expected legacy CLIEngineConfig carried through byte-identically, got %q", resolved.Command)
	}
}

func TestCLIStepRunner_ResolveConfig_NonCLIResolutionErrors(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-anthropic": {provider: "anthropic", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "api", Ref: "anthropic", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	runner := newCLIStepRunner(orch)
	_, _, err := runner.resolveConfig(context.Background(), &models.Task{ProjectID: "proj-1"})
	if err == nil {
		t.Fatal("expected an error when the Router resolves to a non-cli provider")
	}
}

// quotaSandboxRuntime answers the preflight binary/auth checks cleanly
// (both the "command -v" existence check and any auth_check_command) but
// makes the real CLI invocation "fail" with output matching a known quota
// signature, to exercise the cooldown write-back path end-to-end. The real
// invocation is the only command built as the "cd <workdir> && ...; status=$?"
// script (see cli.go's RunCodeStep) — every other bash -lc call in this flow
// is a preflight check and must succeed for the test to reach RunCodeStep.
type quotaSandboxRuntime struct{ commands []string }

func (m *quotaSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	if len(req.Command) >= 3 && strings.Contains(req.Command[2], "status=$?") {
		return &sandbox.CommandResult{ExitCode: 1, Stdout: "Error: usage limit reached, please retry later"}, nil
	}
	return &sandbox.CommandResult{ExitCode: 0}, nil
}
func (m *quotaSandboxRuntime) Health(ctx context.Context) error  { return nil }
func (m *quotaSandboxRuntime) Prewarm(ctx context.Context) error { return nil }
func (m *quotaSandboxRuntime) RunInteractive(ctx context.Context, req sandbox.CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

// fakeCredentialGetter satisfies engine.CredentialGetter so resolveCredentialFiles
// (called during Preflight/RunCodeStep whenever cfg.CredentialID is set) has
// something to resolve instead of erroring out with "no credential service
// configured".
type fakeCredentialGetter struct{}

func (fakeCredentialGetter) GetDecryptedCredential(ctx context.Context, orgID, id string) (string, map[string]string, error) {
	return "cli:claude", map[string]string{}, nil
}

func (fakeCredentialGetter) UpdateCredentialPayload(ctx context.Context, orgID, id string, payload map[string]string) error {
	return nil
}

type fakeCooldownSetter struct {
	ids []string
}

func (f *fakeCooldownSetter) SetCooldown(ctx context.Context, id string, model string, until time.Time) error {
	f.ids = append(f.ids, id)
	return nil
}

type fakeCredStatusSetter struct {
	ids []string
}

func (f *fakeCredStatusSetter) MarkNeedsReauth(ctx context.Context, id string) error {
	f.ids = append(f.ids, id)
	return nil
}

// plainFailureSandboxRuntime answers preflight cleanly, then makes the real
// CLI invocation fail with a non-quota, non-auth error message — used to
// verify RunCLIStep logs the actual failure reason (REQ-001) rather than a
// generic "cli engine finished" message.
type plainFailureSandboxRuntime struct{ commands []string }

func (m *plainFailureSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	if len(req.Command) >= 3 && strings.Contains(req.Command[2], "status=$?") {
		return &sandbox.CommandResult{ExitCode: 1, Stdout: "Not logged in · Please run /login\n"}, nil
	}
	return &sandbox.CommandResult{ExitCode: 0}, nil
}
func (m *plainFailureSandboxRuntime) Health(ctx context.Context) error  { return nil }
func (m *plainFailureSandboxRuntime) Prewarm(ctx context.Context) error { return nil }
func (m *plainFailureSandboxRuntime) RunInteractive(ctx context.Context, req sandbox.CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

func TestCLIStepRunner_RunCLIStep_LogsErrorLevelOnFailure(t *testing.T) {
	rt := &plainFailureSandboxRuntime{}
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	workflows := &mockWorkflowRepo{job: &models.WorkflowJob{}}
	orch := New(nil, workflows, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
		WithCredentials(fakeCredentialGetter{}),
		WithWorkspaceRoot(t.TempDir()),
	)
	runner := newCLIStepRunner(orch)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}

	_, err := runner.RunCLIStep(context.Background(), task, nil, "job-1", "cli_analyze", "analyze the repo", nil, nil, "")
	if err == nil {
		t.Fatal("expected an error since the mocked cli invocation exits non-zero")
	}

	var found bool
	for _, entry := range workflows.logs {
		if entry.Level == "error" && strings.Contains(entry.Message, "Not logged in") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an error-level log entry containing the CLI failure reason, got: %+v", workflows.logs)
	}
}

// authInvalidSandboxRuntime answers preflight cleanly, then makes the real
// CLI invocation fail with an auth-invalid signature.
type authInvalidSandboxRuntime struct{ commands []string }

func (m *authInvalidSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	if len(req.Command) >= 3 && strings.Contains(req.Command[2], "status=$?") {
		return &sandbox.CommandResult{ExitCode: 1, Stdout: "Not logged in · Please run /login\n"}, nil
	}
	return &sandbox.CommandResult{ExitCode: 0}, nil
}
func (m *authInvalidSandboxRuntime) Health(ctx context.Context) error  { return nil }
func (m *authInvalidSandboxRuntime) Prewarm(ctx context.Context) error { return nil }
func (m *authInvalidSandboxRuntime) RunInteractive(ctx context.Context, req sandbox.CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

// awaitingInputSandboxRuntime answers preflight cleanly, then makes the real
// CLI invocation stop mid-run looking like it's waiting for an answer.
type awaitingInputSandboxRuntime struct{ commands []string }

func (m *awaitingInputSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	if len(req.Command) >= 3 && strings.Contains(req.Command[2], "status=$?") {
		return &sandbox.CommandResult{ExitCode: 1, Stdout: "Analyzing repo...\nProceed with deletion? (y/n)"}, nil
	}
	return &sandbox.CommandResult{ExitCode: 0}, nil
}
func (m *awaitingInputSandboxRuntime) Health(ctx context.Context) error  { return nil }
func (m *awaitingInputSandboxRuntime) Prewarm(ctx context.Context) error { return nil }
func (m *awaitingInputSandboxRuntime) RunInteractive(ctx context.Context, req sandbox.CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

// TestCLIStepRunner_RunCLIStep_AwaitingInputLogsAtInfoNotError guards a fixed
// bug: the differentiated success/failure logging (REQ-001) checked
// res.Success before res.AwaitingInput, and AwaitingInput forces
// Success=false — so a routine clarification pause was logged at "error"
// level as "cli engine failed", burying it alongside genuine failures
// instead of being distinguishable as an expected pause.
func TestCLIStepRunner_RunCLIStep_AwaitingInputLogsAtInfoNotError(t *testing.T) {
	rt := &awaitingInputSandboxRuntime{}
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	workflows := &mockWorkflowRepo{job: &models.WorkflowJob{}}
	orch := New(nil, workflows, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
		WithCredentials(fakeCredentialGetter{}),
		WithWorkspaceRoot(t.TempDir()),
	)
	runner := newCLIStepRunner(orch)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}

	out, err := runner.RunCLIStep(context.Background(), task, nil, "job-1", "cli_analyze", "analyze the repo", nil, nil, "")
	if err != nil {
		t.Fatalf("expected no error for an awaiting-input pause, got: %v", err)
	}
	if !out.AwaitingInput {
		t.Fatalf("expected out.AwaitingInput=true, got %+v", out)
	}
	for _, entry := range workflows.logs {
		if entry.Level == "error" {
			t.Errorf("expected no error-level log entries for an awaiting-input pause, got: %+v", entry)
		}
	}
}

func TestCLIStepRunner_RunCLIStep_AuthInvalidSkipsRetry(t *testing.T) {
	rt := &authInvalidSandboxRuntime{}
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
		WithCredentials(fakeCredentialGetter{}),
		WithWorkspaceRoot(t.TempDir()),
	)
	runner := newCLIStepRunner(orch)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}

	credStatus := &fakeCredStatusSetter{}
	orch.credStatusSetter = credStatus

	_, err := runner.RunCLIStep(context.Background(), task, nil, "job-1", "cli_analyze", "analyze the repo", nil, nil, "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, engine.ErrConfigInvalid) {
		t.Errorf("expected error to wrap engine.ErrConfigInvalid so worker.go's retry loop skips remaining retries, got: %v", err)
	}
	if len(credStatus.ids) != 1 || credStatus.ids[0] != "cred-claude" {
		t.Errorf("expected MarkNeedsReauth called once for cred-claude (confirmed match), got %+v", credStatus.ids)
	}
}

// suspectedAuthInvalidSandboxRuntime answers preflight cleanly, then makes
// the real CLI invocation fail with a signature that only the generic "*"
// fallback list covers (not claude_code's own profile-specific rules) —
// e.g. a stray "401 unauthorized" that isn't necessarily a real auth
// failure for this CLI.
type suspectedAuthInvalidSandboxRuntime struct{ commands []string }

func (m *suspectedAuthInvalidSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.commands = append(m.commands, req.Command...)
	if len(req.Command) >= 3 && strings.Contains(req.Command[2], "status=$?") {
		return &sandbox.CommandResult{ExitCode: 1, Stdout: "request failed: 401 unauthorized\n"}, nil
	}
	return &sandbox.CommandResult{ExitCode: 0}, nil
}
func (m *suspectedAuthInvalidSandboxRuntime) Health(ctx context.Context) error  { return nil }
func (m *suspectedAuthInvalidSandboxRuntime) Prewarm(ctx context.Context) error { return nil }
func (m *suspectedAuthInvalidSandboxRuntime) RunInteractive(ctx context.Context, req sandbox.CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

// TestCLIStepRunner_RunCLIStep_SuspectedAuthInvalidRetriesNormally guards
// Option C of the auth-invalid confidence policy: a match on the generic
// "*" fallback list only (not a profile-specific rule) must NOT skip
// retries or disable the credential — it's treated as an ordinary
// retriable failure, with a warning logged instead, since the generic
// patterns (bare "401", "unauthorized") can incidentally match legitimate
// output unrelated to this CLI's actual auth state.
func TestCLIStepRunner_RunCLIStep_SuspectedAuthInvalidRetriesNormally(t *testing.T) {
	rt := &suspectedAuthInvalidSandboxRuntime{}
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	workflows := &mockWorkflowRepo{job: &models.WorkflowJob{}}
	orch := New(nil, workflows, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
		WithCredentials(fakeCredentialGetter{}),
		WithWorkspaceRoot(t.TempDir()),
	)
	credStatus := &fakeCredStatusSetter{}
	orch.credStatusSetter = credStatus
	runner := newCLIStepRunner(orch)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}

	_, err := runner.RunCLIStep(context.Background(), task, nil, "job-1", "cli_analyze", "analyze the repo", nil, nil, "")
	if err == nil {
		t.Fatal("expected an error since the mocked cli invocation exits non-zero")
	}
	if errors.Is(err, engine.ErrConfigInvalid) {
		t.Errorf("expected a plain retriable error, not engine.ErrConfigInvalid, for a merely suspected (fallback-only) auth match, got: %v", err)
	}
	if len(credStatus.ids) != 0 {
		t.Errorf("expected MarkNeedsReauth NOT called for a merely suspected auth match, got %+v", credStatus.ids)
	}
	var foundWarn bool
	for _, entry := range workflows.logs {
		if entry.Level == "warn" && strings.Contains(entry.Message, "suspected auth-invalid") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected a warn-level log entry about the suspected auth-invalid match, got: %+v", workflows.logs)
	}
}

func TestCLIStepRunner_RunCLIStep_QuotaExceededSetsCooldown(t *testing.T) {
	rt := &quotaSandboxRuntime{}
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	cooldown := &fakeCooldownSetter{}
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, rt,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
		WithCooldownSetter(cooldown),
		WithCredentials(fakeCredentialGetter{}),
		WithWorkspaceRoot(t.TempDir()),
	)
	runner := newCLIStepRunner(orch)
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}

	_, err := runner.RunCLIStep(context.Background(), task, nil, "job-1", "cli_analyze", "analyze the repo", nil, nil, "")
	if err == nil {
		t.Fatal("expected an error since the mocked cli invocation exits non-zero")
	}
	if len(cooldown.ids) != 1 || cooldown.ids[0] != "cred-claude" {
		t.Errorf("expected SetCooldown called once with cred-claude, got %v", cooldown.ids)
	}
}
