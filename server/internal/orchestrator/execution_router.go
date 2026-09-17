package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
	GetMinCooldown(ctx context.Context, orgID, provider, model string) (time.Duration, string, error)
}

// CooldownSetter cools a credential down after a CLI run's captured output
// matches a quota/rate-limit signature (REQ-006 write-side). Satisfied by
// *service.CredentialPoolService.SetCooldown.
type CooldownSetter interface {
	SetCooldown(ctx context.Context, id string, model string, until time.Time) error
}

// CredentialStatusSetter marks a credential as needing re-login after a CLI
// run's captured output matches a confirmed "session/token invalid"
// signature (see engine.CodeStepResult.AuthInvalidConfirmed / cli_auth.go).
// Unlike
// CooldownSetter, this failure won't self-resolve with time — the credential
// stays excluded from SelectCredential until a human re-authenticates it.
// Satisfied by *service.CredentialPoolService.MarkNeedsReauth.
type CredentialStatusSetter interface {
	MarkNeedsReauth(ctx context.Context, id string) error
}

// ResolvedExecutionProvider is what ResolveExecutionProvider hands back to
// the caller: enough to build a cliEngineRunner without re-reading
// project.CLIEngineConfig.
//
// This used to carry a Type discriminator ("api" | "cli") because the
// router arbitrated between two engines. There is only one engine now, so
// resolution either yields a CLI candidate or fails — "no provider" is an
// error, never a silent fall-through to an api-native default.
type ResolvedExecutionProvider struct {
	Ref          string
	CredentialID string
	CLIConfig    *models.CLIEngineConfig
}

// ErrNoDistinctHarness is returned by ResolveReviewProvider when every
// available CLI candidate is the same profile that wrote the code. Callers
// must treat it as "skip cross_review" — never as "review with the same
// harness", which is review theater (see docs/openspecs/remove-api-execution-mode).
var ErrNoDistinctHarness = errors.New("no CLI harness distinct from the coder is available")

type ErrAllCredentialsInCooldown struct {
	SleepUntil time.Time
}

func (e *ErrAllCredentialsInCooldown) Error() string {
	return fmt.Sprintf("all credentials in cooldown until %v", e.SleepUntil)
}

// ResolveExecutionProvider picks the first enabled, available CLI candidate
// in priority order, per this precedence chain:
//  1. project.ExecutionProviders, if it has at least one enabled row
//     (models.HasEnabledProvider — see below for why "non-empty" isn't the
//     right check). Exhausting this list is a hard error.
//  2. organization.DefaultExecutionProviders, if non-empty and it produces
//     an available candidate.
//
// There is no step 3. Before the API-native engine was removed this chain
// ended in a plain api_native default, so a project with nothing configured
// still ran; now it fails with an actionable message instead, because
// running without a configured CLI harness is not a thing the platform can
// do.
//
// Selection happens once per call — callers must not re-invoke mid-task to
// "switch" providers (REQ-005).
func (o *Orchestrator) ResolveExecutionProvider(ctx context.Context, task *models.Task, project *models.Project) (*ResolvedExecutionProvider, error) {
	return o.resolveExecutionProvider(ctx, task, project, "")
}

// ResolveReviewProvider resolves a CLI candidate for the cross-harness
// review step, excluding excludeRef — the profile that wrote the code.
// Returns ErrNoDistinctHarness when no other profile is authenticated.
func (o *Orchestrator) ResolveReviewProvider(ctx context.Context, task *models.Task, project *models.Project, excludeRef string) (*ResolvedExecutionProvider, error) {
	resolved, err := o.resolveExecutionProvider(ctx, task, project, excludeRef)
	if err != nil {
		var cooldown *ErrAllCredentialsInCooldown
		if errors.As(err, &cooldown) {
			return nil, err
		}
		return nil, ErrNoDistinctHarness
	}
	return resolved, nil
}

func (o *Orchestrator) resolveExecutionProvider(ctx context.Context, task *models.Task, project *models.Project, excludeRef string) (*ResolvedExecutionProvider, error) {
	providers, err := models.ValidateExecutionProviders(project.ExecutionProviders)
	if err != nil {
		return nil, fmt.Errorf("execution provider routing: %w", err)
	}
	// "Configured" means at least one row is enabled, not merely that the
	// list is non-empty: the Execution Providers UI always persists the
	// full set of known provider rows (each defaulting to enabled:false) on
	// every Project Settings save, whether or not the user touched that
	// section — so project.ExecutionProviders is essentially never
	// byte-empty once a project has been saved even once. Treating "list
	// present but nothing enabled" as an explicit, exhausted configuration
	// would hard-error every such project instead of falling through to the
	// org default.
	if models.HasEnabledProvider(providers) {
		return o.resolveFromProviderList(ctx, project.OrgID, providers, excludeRef)
	}
	if resolved, ok := o.resolveFromOrgDefault(ctx, task, project, excludeRef); ok {
		return resolved, nil
	}
	return nil, fmt.Errorf("no CLI execution provider configured for this project: enable one in Project Settings > Execution Providers and authenticate a CLI credential")
}

// resolveFromProviderList runs the priority-ordered candidate selection
// (sort, availability check) against any provider list — shared by
// project.ExecutionProviders and organization.DefaultExecutionProviders so
// the two sources can never drift in behavior.
//
// excludeRef, when non-empty, skips that profile: used by
// ResolveReviewProvider to guarantee the cross-harness reviewer is not the
// harness that wrote the code.
func (o *Orchestrator) resolveFromProviderList(ctx context.Context, orgID string, providers []models.ExecutionProviderConfig, excludeRef string) (*ResolvedExecutionProvider, error) {
	sorted := make([]models.ExecutionProviderConfig, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	var minSleep time.Time
	for _, p := range sorted {
		if !p.Enabled || (excludeRef != "" && p.Ref == excludeRef) {
			continue
		}
		cfg, credID, isAvail := o.resolveCLICandidate(ctx, orgID, p)
		if isAvail {
			return &ResolvedExecutionProvider{Ref: p.Ref, CredentialID: credID, CLIConfig: cfg}, nil
		}

		credProvider := ""
		if profile, ok := models.ProfileOrEmpty(p.Ref); ok {
			credProvider = profile.CredentialProvider
		}
		if credProvider != "" && o.credentialPool != nil {
			if cdDur, _, err := o.credentialPool.GetMinCooldown(ctx, orgID, credProvider, ""); err == nil && cdDur > 0 {
				cdTime := time.Now().Add(cdDur)
				if minSleep.IsZero() || cdTime.Before(minSleep) {
					minSleep = cdTime
				}
			}
		}
	}

	if !minSleep.IsZero() {
		return nil, &ErrAllCredentialsInCooldown{SleepUntil: minSleep}
	}

	return nil, fmt.Errorf("no enabled CLI execution provider is available")
}

// resolveFromOrgDefault tries organization.DefaultExecutionProviders.
// Returns ok=false for any reason it can't produce a candidate (no org repo
// wired, org lookup failed, list empty/invalid, or list exhausted) — every
// ok=false case falls through to the legacy/api_native default in the
// caller, never an error surfaced to the user, since "no org default
// configured" is the overwhelmingly common case and must stay silent.
func (o *Orchestrator) resolveFromOrgDefault(ctx context.Context, task *models.Task, project *models.Project, excludeRef string) (*ResolvedExecutionProvider, bool) {
	if o.orgs == nil {
		return nil, false
	}
	org, err := o.orgs.GetByID(ctx, project.OrgID)
	if err != nil {
		return nil, false
	}
	providers, err := models.ValidateExecutionProviders(org.DefaultExecutionProviders)
	if err != nil || len(providers) == 0 {
		return nil, false
	}
	resolved, err := o.resolveFromProviderList(ctx, project.OrgID, providers, excludeRef)
	if err != nil {
		return nil, false
	}
	return resolved, true
}

// hasAvailableCredential reports whether provider (a cli:* credential
// provider, e.g. "cli:claude") has at least one usable credential: the pin
// if credentialID is set, otherwise any active/non-cooldown credential for
// that provider.
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
		// A pinned credential_id must actually belong to the provider this
		// row claims (e.g. a claude_code row pinning a cli:codex
		// credential) — otherwise the wrong secret material gets resolved
		// downstream (resolveCredentialFiles mounts by ID alone, with no
		// provider check of its own).
		if resp.Provider != provider {
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

	if p.Ref == "claude_code" && o.orgAllowsAgentWebSearch(ctx, orgID) {
		cfg.Args = appendClaudeAllowedTools(cfg.Args, "WebSearch", "WebFetch")
	}

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

// orgAllowsAgentWebSearch reports the org's AllowAgentWebSearch flag,
// defaulting to false (deny) for any lookup failure — same fail-closed
// convention as the rest of this file's org-lookup helpers
// (resolveFromOrgDefault).
func (o *Orchestrator) orgAllowsAgentWebSearch(ctx context.Context, orgID string) bool {
	if o.orgs == nil {
		return false
	}
	org, err := o.orgs.GetByID(ctx, orgID)
	if err != nil {
		return false
	}
	return org.AllowAgentWebSearch
}

// appendClaudeAllowedTools adds tools to the value that follows
// "--allowedTools" in a claude_code profile's Args (a single
// comma-separated string, e.g. "Read,Edit,Write,Bash" — see
// cli_profiles.go), without mutating the CLIProfiles registry's backing
// array in place (Args is copied into a new slice/string first).
func appendClaudeAllowedTools(args []string, tools ...string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if a == "--allowedTools" && i+1 < len(out) {
			out[i+1] = strings.Join(append(strings.Split(out[i+1], ","), tools...), ",")
			break
		}
	}
	return out
}
