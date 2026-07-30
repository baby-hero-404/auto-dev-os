package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeCred struct {
	provider      string
	status        string
	cooldownUntil *time.Time
}

// fakeCredentialPool implements CredentialAvailability against an in-memory
// set of credentials keyed by ID, so router tests don't need a real DB.
type fakeCredentialPool struct {
	byID  map[string]fakeCred
	calls int
}

func (f *fakeCredentialPool) SelectCredential(ctx context.Context, orgID, provider, model string, strategy service.CredentialStrategy, excludeIDs map[string]bool) (*service.DecryptedCredential, error) {
	f.calls++
	now := time.Now()
	for id, c := range f.byID {
		if c.provider != provider {
			continue
		}
		if c.status != models.ProviderCredentialStatusActive {
			continue
		}
		if c.cooldownUntil != nil && c.cooldownUntil.After(now) {
			continue
		}
		return &service.DecryptedCredential{ID: id, Provider: provider}, nil
	}
	return nil, service.ErrNoCredentialsAvailable
}

func (f *fakeCredentialPool) GetByID(ctx context.Context, id string) (*models.ProviderCredentialResponse, error) {
	c, ok := f.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &models.ProviderCredentialResponse{ID: id, Provider: c.provider, Status: c.status, CooldownUntil: c.cooldownUntil}, nil
}

func execProviders(t *testing.T, list []models.ExecutionProviderConfig) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestResolveExecutionProvider_EmptyFallsBackToLegacy(t *testing.T) {
	orch := New(nil, nil, nil, nil)
	project := &models.Project{
		OrgID:           "org1",
		ExecutionEngine: models.ExecutionEngineCLI,
		CLIEngineConfig: json.RawMessage(`{"command":"claude","args":["-p"]}`),
	}
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "cli" || resolved.CLIConfig == nil || resolved.CLIConfig.Command != "claude" {
		t.Fatalf("expected legacy cli config carried through, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_EmptyAPINativeFallback(t *testing.T) {
	orch := New(nil, nil, nil, nil)
	project := &models.Project{OrgID: "org1", ExecutionEngine: models.ExecutionEngineAPINative}
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" {
		t.Fatalf("expected api fallback, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_PriorityOrder(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: true},
			{Type: "api", Ref: "anthropic", Priority: 2, Enabled: true},
		}),
	}
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "cli" || resolved.Ref != "claude_code" {
		t.Fatalf("expected priority-1 cli candidate selected, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_FallsThroughOnCooldown(t *testing.T) {
	future := time.Now().Add(time.Hour)
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusRateLimited, cooldownUntil: &future},
		"cred-api":    {provider: "anthropic", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: true},
			{Type: "api", Ref: "anthropic", Priority: 2, Enabled: true},
		}),
	}
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" || resolved.Ref != "anthropic" {
		t.Fatalf("expected fallthrough to priority-2 api candidate, got %+v", resolved)
	}
}

// TestResolveExecutionProvider_OnlyDisabledFallsThroughToDefault: a project
// whose ExecutionProviders list has rows present but none enabled (the
// common real-world shape — the UI always persists the full padded row set
// on every save, defaulting new/untouched rows to enabled:false) must be
// treated as "unconfigured", not as an explicit list that's been exhausted.
// See hasEnabledProvider's doc comment on ResolveExecutionProvider for why
// this distinction matters for the org-default fallback to actually work.
func TestResolveExecutionProvider_OnlyDisabledFallsThroughToDefault(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID:           "org1",
		ExecutionEngine: models.ExecutionEngineAPINative,
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: false},
		}),
	}
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" {
		t.Fatalf("expected plain api_native fallback, got %+v", resolved)
	}
}

// TestResolveExecutionProvider_DisabledCandidateNeverSelected: a disabled
// row must still never be picked when a lower-priority row is enabled and
// available — the enabled row wins, the disabled one is just skipped, not
// treated as "nothing configured" (that only applies when *nothing* in the
// list is enabled at all).
func TestResolveExecutionProvider_DisabledCandidateNeverSelected(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
		"cred-api":    {provider: "anthropic", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: false},
			{Type: "api", Ref: "anthropic", Priority: 1, Enabled: true},
		}),
	}
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" || resolved.Ref != "anthropic" {
		t.Fatalf("expected the enabled priority-1 candidate; disabled priority-0 must never be selected, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_NoneAvailable(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: true},
		}),
	}
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err == nil || err.Error() != "no enabled execution provider is available" {
		t.Fatalf("expected explicit no-provider error, got %v", err)
	}
}

func TestResolveExecutionProvider_CLICredentialRateLimited(t *testing.T) {
	future := time.Now().Add(time.Hour)
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-codex": {provider: "cli:codex", status: models.ProviderCredentialStatusRateLimited, cooldownUntil: &future},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "openai_codex", CredentialID: "cred-codex", Priority: 1, Enabled: true},
		}),
	}
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err == nil {
		t.Fatal("expected rate-limited pinned cli credential to make the candidate unavailable")
	}
}

func TestResolveExecutionProvider_PinnedCredentialWrongProviderRejected(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		// Active, but it's a codex credential pinned onto a claude_code row.
		"cred-codex": {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", CredentialID: "cred-codex", Priority: 0, Enabled: true},
		}),
	}
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err == nil {
		t.Fatal("expected a pinned credential belonging to a different provider to be rejected")
	}
}

func TestResolveExecutionProvider_TaskOverrideNarrowsToAPI(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
		"cred-api":    {provider: "anthropic", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		// cli is highest priority, but the task pins api_native, so it
		// must be skipped entirely rather than silently ignored.
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			{Type: "api", Ref: "anthropic", Priority: 1, Enabled: true},
		}),
	}
	apiNative := models.ExecutionEngineAPINative
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{ExecutionEngine: &apiNative}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" || resolved.Ref != "anthropic" {
		t.Fatalf("expected task override to force api-native, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_TaskOverrideNarrowsToCLI(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
		"cred-api":    {provider: "anthropic", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		// api is highest priority, but the task pins cli, so it must be
		// skipped entirely rather than silently ignored.
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "api", Ref: "anthropic", Priority: 0, Enabled: true},
			{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: true},
		}),
	}
	cli := models.ExecutionEngineCLI
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{ExecutionEngine: &cli}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "cli" || resolved.Ref != "claude_code" {
		t.Fatalf("expected task override to force cli, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_TaskOverrideNoMatchingType(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-api": {provider: "anthropic", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "api", Ref: "anthropic", Priority: 0, Enabled: true},
		}),
	}
	cli := models.ExecutionEngineCLI
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{ExecutionEngine: &cli}, project)
	if err == nil {
		t.Fatal("expected error when task forces cli but no cli provider is enabled")
	}
}

func TestShouldUseCLISpecFirstWorkflow_NilProject(t *testing.T) {
	orch := New(nil, nil, nil, nil)
	if orch.shouldUseCLISpecFirstWorkflow(context.Background(), &models.Task{}, nil) {
		t.Fatal("expected false when project could not be loaded")
	}
}

func TestShouldUseCLISpecFirstWorkflow_LegacyFallbackCLI(t *testing.T) {
	orch := New(nil, nil, nil, nil)
	project := &models.Project{ExecutionEngine: models.ExecutionEngineCLI}
	if !orch.shouldUseCLISpecFirstWorkflow(context.Background(), &models.Task{}, project) {
		t.Fatal("expected true for a legacy project with ExecutionEngine=cli and empty ExecutionProviders")
	}
}

func TestShouldUseCLISpecFirstWorkflow_LegacyFallbackAPINative(t *testing.T) {
	orch := New(nil, nil, nil, nil)
	project := &models.Project{ExecutionEngine: models.ExecutionEngineAPINative}
	if orch.shouldUseCLISpecFirstWorkflow(context.Background(), &models.Task{}, project) {
		t.Fatal("expected false for a legacy project with ExecutionEngine=api_native")
	}
}

func TestShouldUseCLISpecFirstWorkflow_ExecutionProvidersOnly(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	// ExecutionEngine stays at its api_native default — only ExecutionProviders is configured.
	project := &models.Project{
		OrgID:           "org1",
		ExecutionEngine: models.ExecutionEngineAPINative,
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
		}),
	}
	if !orch.shouldUseCLISpecFirstWorkflow(context.Background(), &models.Task{}, project) {
		t.Fatal("expected true when ExecutionProviders alone resolves to a cli candidate, even with legacy ExecutionEngine=api_native")
	}
}

func TestShouldUseCLISpecFirstWorkflow_NoCLICandidateAvailable(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
		}),
	}
	if orch.shouldUseCLISpecFirstWorkflow(context.Background(), &models.Task{}, project) {
		t.Fatal("expected false when no cli candidate is actually available")
	}
}

// fakeOrgRepo satisfies OrganizationRepository for org-default fallback tests.
type fakeOrgRepo struct {
	org *models.Organization
	err error
}

func (f *fakeOrgRepo) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.org, nil
}

func TestResolveExecutionProvider_OrgDefaultUsedWhenProjectHasNothing(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orgs := &fakeOrgRepo{org: &models.Organization{
		ID: "org1",
		DefaultExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
		}),
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool), WithOrganizationRepository(orgs))
	// Both untouched defaults: no project-level ExecutionProviders, legacy
	// ExecutionEngine still api_native.
	project := &models.Project{OrgID: "org1", ExecutionEngine: models.ExecutionEngineAPINative}

	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "cli" || resolved.Ref != "claude_code" {
		t.Fatalf("expected org default candidate, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_LegacyCLIBeatsOrgDefault(t *testing.T) {
	// Project already deliberately on the legacy cli path — org default must
	// never override it, even though the org default would resolve fine too.
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orgs := &fakeOrgRepo{org: &models.Organization{
		ID: "org1",
		DefaultExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
		}),
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool), WithOrganizationRepository(orgs))
	cfg := models.CLIEngineConfig{Command: "my-custom-cli"}
	raw, _ := json.Marshal(cfg)
	project := &models.Project{
		OrgID:           "org1",
		ExecutionEngine: models.ExecutionEngineCLI,
		CLIEngineConfig: raw,
	}

	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.CLIConfig == nil || resolved.CLIConfig.Command != "my-custom-cli" {
		t.Fatalf("expected the project's own legacy cli config, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_ProjectListNeverFallsThroughToOrgDefault(t *testing.T) {
	// Project has its own execution_providers, but the one candidate in it
	// is unavailable. Must hard-error, not silently fall through to the org
	// default (REQ-004 of cli-execution-provider-routing: exhausting an
	// explicit list is an error, never a silent fallback).
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-codex": {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orgs := &fakeOrgRepo{org: &models.Organization{
		ID: "org1",
		DefaultExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "openai_codex", Priority: 0, Enabled: true},
		}),
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool), WithOrganizationRepository(orgs))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true}, // no cred-claude in pool
		}),
	}

	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err == nil {
		t.Fatal("expected a hard error when the project's own list is exhausted, not a fall-through to org default")
	}
}

func TestResolveExecutionProvider_NeitherConfiguredIsPlainAPINative(t *testing.T) {
	orgs := &fakeOrgRepo{org: &models.Organization{ID: "org1"}} // DefaultExecutionProviders empty
	orch := New(nil, nil, nil, nil, WithOrganizationRepository(orgs))
	project := &models.Project{OrgID: "org1", ExecutionEngine: models.ExecutionEngineAPINative}

	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" {
		t.Fatalf("expected plain api_native fallback, got %+v", resolved)
	}
}

func TestResolveExecutionProvider_NoOrgRepoWiredFallsBackSilently(t *testing.T) {
	// No WithOrganizationRepository at all (e.g. some unit tests, or a
	// deployment that hasn't wired it) — must not panic or error, just fall
	// through as if no org default existed.
	orch := New(nil, nil, nil, nil)
	project := &models.Project{OrgID: "org1", ExecutionEngine: models.ExecutionEngineAPINative}

	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "api" {
		t.Fatalf("expected plain api_native fallback, got %+v", resolved)
	}
}
