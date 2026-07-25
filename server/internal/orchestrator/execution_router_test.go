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

func TestResolveExecutionProvider_DisabledSkipped(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil, WithCredentialAvailability(pool))
	project := &models.Project{
		OrgID: "org1",
		ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: false},
		}),
	}
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, project)
	if err == nil {
		t.Fatal("expected error, disabled candidate must never be selected")
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
