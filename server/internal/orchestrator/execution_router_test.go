package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// GetMinCooldown mirrors *service.CredentialPoolService.GetMinCooldown's
// semantics against the fake's in-memory set: the shortest remaining
// cooldown among matching-provider credentials, or 0 if any of them is
// free right now.
func (f *fakeCredentialPool) GetMinCooldown(ctx context.Context, orgID, provider, model string) (time.Duration, string, error) {
	now := time.Now()
	var minCooldown time.Duration = -1
	var minCredID string
	for id, c := range f.byID {
		if c.provider != provider {
			continue
		}
		var cd time.Duration
		if c.cooldownUntil != nil && c.cooldownUntil.After(now) {
			cd = c.cooldownUntil.Sub(now)
		}
		if minCooldown == -1 || cd < minCooldown {
			minCooldown = cd
			minCredID = id
		}
	}
	if minCooldown == -1 {
		return 0, "", nil
	}
	return minCooldown, minCredID, nil
}

func execProviders(t *testing.T, list []models.ExecutionProviderConfig) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	return raw
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

func TestResolveExecutionProvider_PriorityOrder(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
		"cred-codex":  {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "openai_codex", Priority: 1, Enabled: true},
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Ref != "claude_code" {
		t.Errorf("expected lowest-priority-number candidate claude_code, got %q", resolved.Ref)
	}
}

func TestResolveExecutionProvider_FallsThroughOnUnavailable(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-codex": {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
				{Type: "cli", Ref: "openai_codex", Priority: 1, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Ref != "openai_codex" {
		t.Errorf("expected fall-through to the authenticated candidate, got %q", resolved.Ref)
	}
}

func TestResolveExecutionProvider_DisabledCandidateNeverSelected(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: false},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	if err == nil {
		t.Fatal("expected an error: the only row is disabled, so nothing is configured")
	}
}

// Nothing configured is now a hard error. It used to fall through to an
// api-native default, which is the behavior this whole change removes: the
// platform cannot run a task without a CLI harness, so it must say so.
func TestResolveExecutionProvider_NothingConfiguredIsAnError(t *testing.T) {
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{OrgID: "org1"}}),
	)
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	if err == nil {
		t.Fatal("expected an error when no CLI execution provider is configured")
	}
	if !strings.Contains(err.Error(), "no CLI execution provider configured") {
		t.Errorf("expected an actionable message naming the missing config, got %q", err.Error())
	}
}

func TestResolveExecutionProvider_AllCredentialsInCooldown(t *testing.T) {
	cooldownUntil := time.Now().Add(5 * time.Minute)
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive, cooldownUntil: &cooldownUntil},
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
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	var cooldown *ErrAllCredentialsInCooldown
	if !errors.As(err, &cooldown) {
		t.Fatalf("expected ErrAllCredentialsInCooldown so the job sleeps and retries, got %v", err)
	}
}

func TestResolveExecutionProvider_PinnedCredentialWrongProviderRejected(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-codex": {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", CredentialID: "cred-codex", Priority: 0, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	_, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	if err == nil {
		t.Fatal("expected a claude_code row pinning a cli:codex credential to be rejected")
	}
}

// ResolveReviewProvider must never hand back the harness that wrote the code:
// a model reviewing its own output shares its blind spots.
func TestResolveReviewProvider_ExcludesCoderHarness(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-claude": {provider: "cli:claude", status: models.ProviderCredentialStatusActive},
		"cred-codex":  {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
				{Type: "cli", Ref: "openai_codex", Priority: 1, Enabled: true},
			}),
		}}),
		WithCredentialAvailability(pool),
	)
	resolved, err := orch.ResolveReviewProvider(context.Background(), &models.Task{}, mustProject(t, orch), "claude_code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Ref != "openai_codex" {
		t.Errorf("expected the reviewer to be a different harness, got %q", resolved.Ref)
	}
}

func TestResolveReviewProvider_SingleHarnessReturnsSentinel(t *testing.T) {
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
	_, err := orch.ResolveReviewProvider(context.Background(), &models.Task{}, mustProject(t, orch), "claude_code")
	if !errors.Is(err, ErrNoDistinctHarness) {
		t.Fatalf("expected ErrNoDistinctHarness so cross_review is skipped rather than self-reviewed, got %v", err)
	}
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
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{OrgID: "org1"}}),
		WithOrganizationRepository(orgs),
		WithCredentialAvailability(pool),
	)
	resolved, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Ref != "claude_code" {
		t.Errorf("expected the org default to supply the candidate, got %q", resolved.Ref)
	}
}

// A project with its own enabled list owns the decision outright: exhausting
// it is a hard error, never a silent slide into the org default.
func TestResolveExecutionProvider_ProjectListNeverFallsThroughToOrgDefault(t *testing.T) {
	pool := &fakeCredentialPool{byID: map[string]fakeCred{
		"cred-codex": {provider: "cli:codex", status: models.ProviderCredentialStatusActive},
	}}
	orgs := &fakeOrgRepo{org: &models.Organization{
		ID: "org1",
		DefaultExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
			{Type: "cli", Ref: "openai_codex", Priority: 0, Enabled: true},
		}),
	}}
	orch := New(nil, nil, nil, nil,
		WithProjectRepository(&fakeProjectRepo{project: &models.Project{
			OrgID: "org1",
			ExecutionProviders: execProviders(t, []models.ExecutionProviderConfig{
				{Type: "cli", Ref: "claude_code", Priority: 0, Enabled: true},
			}),
		}}),
		WithOrganizationRepository(orgs),
		WithCredentialAvailability(pool),
	)
	if _, err := orch.ResolveExecutionProvider(context.Background(), &models.Task{}, mustProject(t, orch)); err == nil {
		t.Fatal("expected an exhausted project list to error rather than use the org default")
	}
}

// mustProject reads the project back out of the fake repo the orchestrator was
// built with, so each test states its project once.
func mustProject(t *testing.T, o *Orchestrator) *models.Project {
	t.Helper()
	p, err := o.projects.GetByID(context.Background(), "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
