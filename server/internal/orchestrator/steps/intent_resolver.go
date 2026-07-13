package steps

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// IntentResolutionError is a structured, per-node resolution failure (REQ-004).
type IntentResolutionError struct {
	NodeID     string
	Capability string
	Reason     string
}

func (e *IntentResolutionError) Error() string {
	return fmt.Sprintf("intent resolver: node %q capability %q: %s", e.NodeID, e.Capability, e.Reason)
}

// intentTokens splits a capability identifier — "UserRepository",
// "user_repository", "user-repository" — into lowercase match tokens.
func intentTokens(capability string) []string {
	runes := []rune(capability)
	var b strings.Builder
	for i, r := range runes {
		if r == '_' || r == '-' || r == ' ' || r == '/' {
			b.WriteRune(' ')
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			// Boundary before a capital following lowercase/digit ("userR" -> "user R"),
			// or before the last capital of an acronym run ("APIC[lient]" -> "API Client").
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
				b.WriteRune(' ')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}

	fields := strings.Fields(b.String())
	tokens := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if len(f) < 2 || seen[f] {
			continue
		}
		seen[f] = true
		tokens = append(tokens, f)
	}
	return tokens
}

// pathMatchesTokens reports whether every token appears somewhere in path
// (case-insensitive substring match).
func pathMatchesTokens(path string, tokens []string) bool {
	lower := strings.ToLower(path)
	for _, t := range tokens {
		if !strings.Contains(lower, t) {
			return false
		}
	}
	return true
}

// ResolveIntent maps a single ExecutionIR's semantic intent to physical
// workspace paths by matching its capability tokens against candidates —
// typically analysis.AffectedFiles, which the Planner already derived from
// the repository (for both existing files and proposed new ones). The
// resolver itself performs no I/O (design.md § Security: Intent Resolver is
// Workspace/Read-only — it only inspects data already read by the Planner).
//
// Multiple matches are a legitimate success (REQ-004 does not treat ambiguity
// as failure): a capability may legitimately span several files. Zero matches
// is the only failure mode.
func ResolveIntent(ir models.ExecutionIR, candidates []models.AffectedFile) ([]string, error) {
	tokens := intentTokens(ir.Intent.Capability)
	if len(tokens) == 0 {
		return nil, &IntentResolutionError{
			NodeID:     ir.NodeID,
			Capability: ir.Intent.Capability,
			Reason:     "empty capability, cannot derive match tokens",
		}
	}

	var matches []string
	for _, c := range candidates {
		if c.File != "" && pathMatchesTokens(c.File, tokens) {
			matches = append(matches, c.File)
		}
	}

	if len(matches) == 0 {
		return nil, &IntentResolutionError{
			NodeID:     ir.NodeID,
			Capability: ir.Intent.Capability,
			Reason:     fmt.Sprintf("no workspace file matched tokens [%s]", strings.Join(tokens, ", ")),
		}
	}
	return matches, nil
}

// ResolveExecutionIRTargets resolves physical targets for every ExecutionIR
// in the analysis against analysis.AffectedFiles. It returns the resolved
// map (node_id -> paths) for every IR that did resolve, plus an aggregated
// error (via errors.Join) naming every IR that didn't, so callers can decide
// how to react to partial resolution.
//
// NOTE: this does not yet block PLAN_READY -> IMPLEMENTATION (REQ-004's
// fail-fast scenario) because that transition doesn't exist until the Node
// State Machine (Task 2.1) lands. PlanStep currently logs unresolved intents
// as warnings rather than pausing the workflow, consistent with the
// flag-gated rollout in design.md § Migration & Rollout — hard enforcement
// activates alongside execution.state_machine_enabled.
func ResolveExecutionIRTargets(analysis models.TaskAnalysis) (map[string][]string, error) {
	resolved := make(map[string][]string, len(analysis.ExecutionIRs))
	var errs []error
	for _, ir := range analysis.ExecutionIRs {
		targets, err := ResolveIntent(ir, analysis.AffectedFiles)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		resolved[ir.NodeID] = targets
	}
	if len(errs) > 0 {
		return resolved, errors.Join(errs...)
	}
	return resolved, nil
}
