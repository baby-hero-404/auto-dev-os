package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// CredentialAvailability abstracts the credential pool lookups the Execution
// Router needs: is there an active, non-cooling-down credential for a given
// provider (auto-pick), or is a specific pinned credential itself active.
// Method signatures match *service.CredentialPoolService exactly (no
// adapter needed) — SelectCredential is the same lookup CredentialPoolService
// already uses at LLM-call time, so "available" here means the exact same
// thing it means everywhere else in the codebase.
type CredentialAvailability interface {
	SelectCredential(ctx context.Context, orgID, provider, model string, strategy service.CredentialStrategy, excludeIDs map[string]bool) (*service.DecryptedCredential, error)
	GetByID(ctx context.Context, id string) (*models.ProviderCredentialResponse, error)
}

// CooldownSetter cools a credential down after a CLI run's captured output
// matches a quota/rate-limit signature (REQ-006 write-side). Satisfied by
// *service.CredentialPoolService.SetCooldown.
type CooldownSetter interface {
	SetCooldown(ctx context.Context, id string, model string, until time.Time) error
}

// ResolvedExecutionProvider is what ResolveExecutionProvider hands back to
// the caller: enough to either use the existing api-native LLM path, or
// build a cliEngineRunner without re-reading project.CLIEngineConfig.
type ResolvedExecutionProvider struct {
	Type         string // "api" | "cli"
	Ref          string
	CredentialID string                  // resolved concrete credential id (cli); empty for api (CredentialPoolService picks at call time)
	CLIConfig    *models.CLIEngineConfig // populated for type=="cli"
}

// ResolveExecutionProvider picks the first enabled, available candidate from
// project.ExecutionProviders in priority order (REQ-004). An empty/absent
// ExecutionProviders falls back to the legacy ExecutionEngine/CLIEngineConfig
// behavior unchanged (REQ-003). Selection happens once per call — callers
// must not re-invoke mid-task to "switch" providers (REQ-005).
func (o *Orchestrator) ResolveExecutionProvider(ctx context.Context, task *models.Task, project *models.Project) (*ResolvedExecutionProvider, error) {
	providers, err := models.ValidateExecutionProviders(project.ExecutionProviders)
	if err != nil {
		return nil, fmt.Errorf("execution provider routing: %w", err)
	}
	if len(providers) == 0 {
		return o.legacyResolveExecutionProvider(task, project), nil
	}

	sorted := make([]models.ExecutionProviderConfig, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	for _, p := range sorted {
		if !p.Enabled {
			continue
		}
		switch p.Type {
		case "api":
			if o.hasAvailableCredential(ctx, project.OrgID, p.Ref, p.CredentialID) {
				return &ResolvedExecutionProvider{Type: "api", Ref: p.Ref}, nil
			}
		case "cli":
			cfg, credID, ok := o.resolveCLICandidate(ctx, project.OrgID, p)
			if ok {
				return &ResolvedExecutionProvider{Type: "cli", Ref: p.Ref, CredentialID: credID, CLIConfig: cfg}, nil
			}
		}
	}
	return nil, fmt.Errorf("no enabled execution provider is available")
}

// legacyResolveExecutionProvider reuses engine.ResolveEngine + the existing
// project.CLIEngineConfig read verbatim, so REQ-003 can never drift from
// today's behavior (it's the same code path, not a reimplementation).
func (o *Orchestrator) legacyResolveExecutionProvider(task *models.Task, project *models.Project) *ResolvedExecutionProvider {
	resolved := engine.ResolveEngine(task.ExecutionEngine, project.ExecutionEngine)
	if resolved != models.ExecutionEngineCLI {
		return &ResolvedExecutionProvider{Type: "api"}
	}
	cfg := legacyCLIEngineConfig(project)
	cfg.ProfileRef = "custom"
	return &ResolvedExecutionProvider{Type: "cli", Ref: "custom", CredentialID: cfg.CredentialID, CLIConfig: cfg}
}

// legacyCLIEngineConfig parses project.CLIEngineConfig exactly as
// resolveCLIEngineRunner did before this OpenSpec — byte-identical
// unmarshal, no reinterpretation (REQ-003).
func legacyCLIEngineConfig(project *models.Project) *models.CLIEngineConfig {
	var cfg models.CLIEngineConfig
	if len(project.CLIEngineConfig) > 0 {
		_ = json.Unmarshal(project.CLIEngineConfig, &cfg)
	}
	return &cfg
}

// hasAvailableCredential reports whether provider (api provider id, e.g.
// "anthropic") has at least one usable credential: the pin if credentialID
// is set, otherwise any active/non-cooldown credential for that provider.
func (o *Orchestrator) hasAvailableCredential(ctx context.Context, orgID, provider, credentialID string) bool {
	if o.credentialPool == nil {
		// No credential pool wired (e.g. some unit tests) — don't block
		// routing on a dependency that was never configured.
		return true
	}
	if credentialID != "" {
		resp, err := o.credentialPool.GetByID(ctx, credentialID)
		if err != nil {
			return false
		}
		return resp.Status == models.ProviderCredentialStatusActive && (resp.CooldownUntil == nil || resp.CooldownUntil.Before(time.Now()))
	}
	_, err := o.credentialPool.SelectCredential(ctx, orgID, provider, "", service.StrategyFillFirst, nil)
	return err == nil
}

// resolveCLICandidate builds the CLIEngineConfig for one "cli" candidate
// (from CLIProfiles[ref], or p.CLIConfig for ref=="custom") and resolves
// which credential it will run with: the pin if set, otherwise the first
// active credential for CLIProfiles[ref].CredentialProvider.
func (o *Orchestrator) resolveCLICandidate(ctx context.Context, orgID string, p models.ExecutionProviderConfig) (*models.CLIEngineConfig, string, bool) {
	var cfg models.CLIEngineConfig
	credentialProvider := ""

	if p.Ref == "custom" {
		if p.CLIConfig == nil {
			return nil, "", false
		}
		cfg = *p.CLIConfig
	} else {
		profile, ok := models.ProfileOrEmpty(p.Ref)
		if !ok {
			return nil, "", false
		}
		cfg = models.CLIEngineConfig{
			Command:          profile.Command,
			Args:             profile.Args,
			AuthCheckCommand: profile.AuthCheckCommand,
			TimeoutMinutes:   profile.TimeoutMinutes,
		}
		credentialProvider = profile.CredentialProvider
	}
	cfg.ProfileRef = p.Ref

	credID := p.CredentialID
	if credID != "" {
		if !o.hasAvailableCredential(ctx, orgID, credentialProvider, credID) {
			return nil, "", false
		}
		cfg.CredentialID = credID
		return &cfg, credID, true
	}

	// custom without a pinned credential is rejected at validation time
	// (models.ValidateExecutionProviders), so credentialProvider is always
	// non-empty here.
	if o.credentialPool == nil {
		cfg.CredentialID = ""
		return &cfg, "", true
	}
	cred, err := o.credentialPool.SelectCredential(ctx, orgID, credentialProvider, "", service.StrategyFillFirst, nil)
	if err != nil {
		return nil, "", false
	}
	cfg.CredentialID = cred.ID
	return &cfg, cred.ID, true
}
