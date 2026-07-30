package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

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

	_, err := runner.RunCLIStep(context.Background(), task, nil, "job-1", "cli_analyze", "analyze the repo", nil, nil)
	if err == nil {
		t.Fatal("expected an error since the mocked cli invocation exits non-zero")
	}
	if len(cooldown.ids) != 1 || cooldown.ids[0] != "cred-claude" {
		t.Errorf("expected SetCooldown called once with cred-claude, got %v", cooldown.ids)
	}
}
