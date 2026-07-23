// Package governance implements declarative, per-project pipeline/policy
// configuration (P4.2): schema-validated JSON stored on models.Project's
// PipelineConfig column, consulted at a small number of existing decision
// points (DoR bypass, review skip, smart-router override) rather than
// rebuilding the workflow DAG from scratch. A nil/empty config means "use
// built-in defaults" (REQ-002) — every accessor below treats a nil *Config
// as a no-op, so unconfigured projects behave exactly as before this
// feature.
package governance

import "strings"

// CurrentVersion is the only config version this build understands (REQ-006).
// A config with a different version is rejected at validation time; when a
// migration path is added for a future version bump, this becomes the
// upper bound a migrate-on-read step resolves toward.
const CurrentVersion = 1

// Config is the parsed shape of Project.PipelineConfig, matching design.md.
type Config struct {
	Version  int       `json:"version"`
	Pipeline *Pipeline `json:"pipeline,omitempty"`
	Policies *Policies `json:"policies,omitempty"`
}

// Pipeline holds patch-style overrides against a preset ("extends"), or (in
// the advanced case) a fully custom step list validated structurally by
// ValidateDAG but not yet wired into a data-driven builder (see
// docs/implementation/declarative-governance-schemas-notes.md).
type Pipeline struct {
	Extends string         `json:"extends,omitempty"`
	Steps   []StepOverride `json:"steps,omitempty"`
}

// StepOverride patches one step of the base pipeline. Enabled=false disables
// a step; SkipWhen conditionally disables it per-task. DependsOn is only
// meaningful when a step declares a full custom graph (ID+DependsOn present
// on every step, no Extends) — see ValidateDAG.
type StepOverride struct {
	ID        string    `json:"id"`
	Enabled   *bool     `json:"enabled,omitempty"`
	SkipWhen  *SkipWhen `json:"skip_when,omitempty"`
	DependsOn []string  `json:"dependsOn,omitempty"`
}

// SkipWhen is a single-condition guard: currently only a label match is
// supported (REQ-003's stated scenario), matched case-insensitively.
type SkipWhen struct {
	Label string `json:"label,omitempty"`
}

// Policies holds project-level overrides consumed directly by policy code
// (smart router, review harness, DoR gate) instead of the hard-coded
// project columns those consumers used before this feature.
type Policies struct {
	Routing            map[string]string `json:"routing,omitempty"`
	ReviewHarness      string            `json:"review_harness,omitempty"`
	MaxReviewFixCycles *int              `json:"max_review_fix_cycles,omitempty"`
	Dor                *DorPolicy        `json:"dor,omitempty"`
}

// DorPolicy overrides the definition-of-ready gate.
type DorPolicy struct {
	Disabled                  bool  `json:"disabled,omitempty"`
	RequireAcceptanceCriteria *bool `json:"require_acceptance_criteria,omitempty"`
}

// IsDorDisabled reports whether cfg disables the DoR gate outright. A nil
// cfg (REQ-002) or nil Policies/Dor always returns false — the gate's
// existing hotfix/clarification-round bypass in policy.IsDefinitionOfReadyBypassed
// is untouched by this.
func (c *Config) IsDorDisabled() bool {
	if c == nil || c.Policies == nil || c.Policies.Dor == nil {
		return false
	}
	return c.Policies.Dor.Disabled
}

// ShouldSkipStepForLabels reports whether stepID has a skip_when.label
// override matching any of labels (case-insensitive). A nil cfg or no
// matching override returns false.
func (c *Config) ShouldSkipStepForLabels(stepID string, labels []string) bool {
	if c == nil || c.Pipeline == nil {
		return false
	}
	for _, so := range c.Pipeline.Steps {
		if so.ID != stepID || so.SkipWhen == nil || so.SkipWhen.Label == "" {
			continue
		}
		for _, l := range labels {
			if strings.EqualFold(strings.TrimSpace(l), so.SkipWhen.Label) {
				return true
			}
		}
	}
	return false
}

// IsStepDisabled reports whether stepID is unconditionally disabled
// (enabled: false) in cfg.
func (c *Config) IsStepDisabled(stepID string) bool {
	if c == nil || c.Pipeline == nil {
		return false
	}
	for _, so := range c.Pipeline.Steps {
		if so.ID == stepID && so.Enabled != nil && !*so.Enabled {
			return true
		}
	}
	return false
}

// RoutingOverride returns the routing-matrix override for stepID, if any
// (REQ-004). ok is false when cfg is nil or has no override for this step,
// in which case callers keep using the built-in matrix/default unchanged.
func (c *Config) RoutingOverride(stepID string) (level string, ok bool) {
	if c == nil || c.Policies == nil || c.Policies.Routing == nil {
		return "", false
	}
	level, ok = c.Policies.Routing[stepID]
	return level, ok
}

// MaxReviewFixCyclesOverride returns the configured override, if any.
func (c *Config) MaxReviewFixCyclesOverride() (int, bool) {
	if c == nil || c.Policies == nil || c.Policies.MaxReviewFixCycles == nil {
		return 0, false
	}
	return *c.Policies.MaxReviewFixCycles, true
}

// ReviewHarnessOverride returns the configured override, if any.
func (c *Config) ReviewHarnessOverride() (string, bool) {
	if c == nil || c.Policies == nil || c.Policies.ReviewHarness == "" {
		return "", false
	}
	return c.Policies.ReviewHarness, true
}
