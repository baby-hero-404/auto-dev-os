package prompt

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestFilterRulesForStep_CodingStep_ExcludesImpossible(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Always validate inputs.", Enforcement: "strict"},
		{ID: "r2", Scope: "global", Content: "Run tests before committing.", Enforcement: "strict"},
		{ID: "r3", Scope: "global", Content: "Use Socratic Gate for features.", Enforcement: "strict"},
		{ID: "r4", Scope: "global", Content: "Secrets must be in environment variables.", Enforcement: "strict"},
		{ID: "r5", Scope: "global", Content: "Ask the user before proceeding.", Enforcement: "strict"},
	}
	got := filterRulesForStep(rules, "code_backend_0")
	// r2 (run tests), r3 (Socratic Gate), r5 (ask the user) should be excluded
	if len(got) != 2 {
		t.Fatalf("expected 2 rules after filtering, got %d: %#v", len(got), got)
	}
	if got[0].ID != "r1" || got[1].ID != "r4" {
		t.Fatalf("unexpected filtered rules: %#v", got)
	}
}

func TestFilterRulesForStep_NonCodingStep_KeepsAll(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Always validate inputs.", Enforcement: "strict"},
		{ID: "r2", Scope: "global", Content: "Run tests before committing.", Enforcement: "strict"},
		{ID: "r3", Scope: "global", Content: "Use Socratic Gate for features.", Enforcement: "strict"},
	}
	got := filterRulesForStep(rules, "analyze")
	if len(got) != 3 {
		t.Fatalf("expected all 3 rules kept for analyze step, got %d", len(got))
	}
}

func TestFilterRulesForStep_EmptyStepID_KeepsAll(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Run tests.", Enforcement: "strict"},
	}
	got := filterRulesForStep(rules, "")
	if len(got) != 1 {
		t.Fatalf("expected all rules kept for empty stepID, got %d", len(got))
	}
}

func TestFilterRulesForStep_FixStep_Filters(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "JIT Knowledge protocol.", Enforcement: "strict"},
		{ID: "r2", Scope: "global", Content: "Keep code clean.", Enforcement: "strict"},
	}
	got := filterRulesForStep(rules, "fix")
	if len(got) != 1 {
		t.Fatalf("expected 1 rule after filtering for fix step, got %d", len(got))
	}
	if got[0].ID != "r2" {
		t.Fatalf("unexpected rule: %#v", got[0])
	}
}
