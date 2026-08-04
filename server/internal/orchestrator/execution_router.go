package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
// the caller: enough to either use the existing api-native LLM path, or
// build a cliEngineRunner without re-reading project.CLIEngineConfig.
type ResolvedExecutionProvider struct {
	Type         string // "api" | "cli"
	Ref          string
	CredentialID string                  // resolved concrete credential id (cli); empty for api (CredentialPoolService picks at call time)
	CLIConfig    *models.CLIEngineConfig // populated for type=="cli"
}

// ResolveExecutionProvider picks the first enabled, available candidate in
// priority order, per this precedence chain (docs/openspecs/global-execution-providers):
//  1. project.ExecutionProviders, if it has at least one enabled row
//     (models.HasEnabledProvider — see below for why "non-empty" isn't the
//     right check) (REQ-004) — exhausting this list is a hard error, never
//     a silent fall-through to steps below.
//  2. project.ExecutionEngine=="cli" (legacy single-engine field), if
//     explicitly set — a project already deliberately on the legacy path
//     keeps running exactly as before, checked *before* the org default so
//     an org enabling a global default can never change its behavior.
//  3. organization.DefaultExecutionProviders, if non-empty and it produces
//     an available candidate.
//  4. Plain api_native default (today's behavior).
//
// Selection happens once per call — callers must not re-invoke mid-task to
// "switch" providers (REQ-005).
//
// A per-task execution_engine override (task.ExecutionEngine) narrows
// whichever list above is selected down to just that type instead of being
// ignored: without this, once a project has any provider routing
// configured, the per-task "API-native" / "CLI" override in the task UI
// silently stopped doing anything, since only the legacy (empty-list) path
// ever consulted it.
func (o *Orchestrator) ResolveExecutionProvider(ctx context.Context, task *models.Task, project *models.Project) (*ResolvedExecutionProvider, error) {
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
	// org default (or legacy), which defeats this feature (and was already
	// a latent bug for the legacy fallback before the org default existed).
	if models.HasEnabledProvider(providers) {
		return o.resolveFromProviderList(ctx, project.OrgID, providers, task.ExecutionEngine)
	}
	if project.ExecutionEngine != models.ExecutionEngineCLI {
		if resolved, ok := o.resolveFromOrgDefault(ctx, task, project); ok {
			return resolved, nil
		}
	}
	return o.legacyResolveExecutionProvider(task, project), nil
}

// resolveFromProviderList runs the priority-ordered candidate selection
// (sort, task-override narrowing, availability check) against any provider
// list — shared by project.ExecutionProviders and
// organization.DefaultExecutionProviders so the two sources can never drift
// in behavior.
func (o *Orchestrator) resolveFromProviderList(ctx context.Context, orgID string, providers []models.ExecutionProviderConfig, taskEngineOverride *string) (*ResolvedExecutionProvider, error) {
	sorted := make([]models.ExecutionProviderConfig, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	if taskEngineOverride != nil && *taskEngineOverride != "" {
		wantType := "api"
		if *taskEngineOverride == models.ExecutionEngineCLI {
			wantType = "cli"
		}
		filtered := make([]models.ExecutionProviderConfig, 0, len(sorted))
		for _, p := range sorted {
			if p.Type == wantType {
				filtered = append(filtered, p)
			}
		}
		sorted = filtered
	}

	for _, p := range sorted {
		if !p.Enabled {
			continue
		}
		switch p.Type {
		case "api":
			if o.hasAvailableCredential(ctx, orgID, p.Ref, p.CredentialID) {
				return &ResolvedExecutionProvider{Type: "api", Ref: p.Ref}, nil
			}
		case "cli":
			cfg, credID, ok := o.resolveCLICandidate(ctx, orgID, p)
			if ok {
				return &ResolvedExecutionProvider{Type: "cli", Ref: p.Ref, CredentialID: credID, CLIConfig: cfg}, nil
			}
		}
	}
	return nil, fmt.Errorf("no enabled execution provider is available")
}

// resolveFromOrgDefault tries organization.DefaultExecutionProviders.
// Returns ok=false for any reason it can't produce a candidate (no org repo
// wired, org lookup failed, list empty/invalid, or list exhausted) — every
// ok=false case falls through to the legacy/api_native default in the
// caller, never an error surfaced to the user, since "no org default
// configured" is the overwhelmingly common case and must stay silent.
func (o *Orchestrator) resolveFromOrgDefault(ctx context.Context, task *models.Task, project *models.Project) (*ResolvedExecutionProvider, bool) {
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
	resolved, err := o.resolveFromProviderList(ctx, project.OrgID, providers, task.ExecutionEngine)
	if err != nil {
		return nil, false
	}
	return resolved, true
}

// shouldUseCLISpecFirstWorkflow reports whether task should run the
// cli_analyze -> cli_spec -> cli_implement workflow shape instead of the
// default DAG. Delegates entirely to ResolveExecutionProvider, which
// already encodes the full precedence chain (project list, legacy-if-cli,
// org default, plain api_native) — this asks the Router the same question
// it will answer again per-step later; both calls happen at task/job start
// with no state change in between, consistent with "resolve once at Task
// start" (REQ-005).
func (o *Orchestrator) shouldUseCLISpecFirstWorkflow(ctx context.Context, task *models.Task, project *models.Project) bool {
	if project == nil {
		return false
	}
	resolved, err := o.ResolveExecutionProvider(ctx, task, project)
	return err == nil && resolved.Type == "cli"
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
		// A pinned credential_id must actually belong to the provider this
		// row claims (e.g. a claude_code row pinning a cli:codex or API
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
