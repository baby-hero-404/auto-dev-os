package prompts

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestFilterRulesForAgent_CodingStep_ExcludesImpossible(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Always validate inputs.", Enforcement: "strict"},
		{ID: "r2", Scope: "global", Content: "Run tests before committing.", Enforcement: "strict"},
		{ID: "r3", Scope: "global", Content: "Use Socratic Gate for features.", Enforcement: "strict"},
		{ID: "r4", Scope: "global", Content: "Secrets must be in environment variables.", Enforcement: "strict"},
		{ID: "r5", Scope: "global", Content: "Ask the user before proceeding.", Enforcement: "strict"},
	}
	got := filterRulesForAgent(rules, nil, "code_backend_0")
	// r2 (run tests), r3 (Socratic Gate), r5 (ask the user) should be excluded
	if len(got) != 2 {
		t.Fatalf("expected 2 rules after filtering, got %d: %#v", len(got), got)
	}
	if got[0].ID != "r1" || got[1].ID != "r4" {
		t.Fatalf("unexpected filtered rules: %#v", got)
	}
}

func TestFilterRulesForAgent_NonCodingStep_KeepsAll(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Always validate inputs.", Enforcement: "strict"},
		{ID: "r2", Scope: "global", Content: "Run tests before committing.", Enforcement: "strict"},
		{ID: "r3", Scope: "global", Content: "Use Socratic Gate for features.", Enforcement: "strict"},
	}
	got := filterRulesForAgent(rules, nil, "analyze")
	if len(got) != 3 {
		t.Fatalf("expected all 3 rules kept for analyze step, got %d", len(got))
	}
}

func TestFilterRulesForAgent_EmptyStepID_KeepsAll(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Run tests.", Enforcement: "strict"},
	}
	got := filterRulesForAgent(rules, nil, "")
	if len(got) != 1 {
		t.Fatalf("expected all rules kept for empty stepID, got %d", len(got))
	}
}

func TestFilterRulesForAgent_FixStep_Filters(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "JIT Knowledge protocol.", Enforcement: "strict"},
		{ID: "r2", Scope: "global", Content: "Keep code clean.", Enforcement: "strict"},
	}
	got := filterRulesForAgent(rules, nil, "fix")
	if len(got) != 1 {
		t.Fatalf("expected 1 rule after filtering for fix step, got %d", len(got))
	}
	if got[0].ID != "r2" {
		t.Fatalf("unexpected rule: %#v", got[0])
	}
}

func TestFilterRulesForAgent_RoleSpecificFiltering(t *testing.T) {
	rules := []models.Rule{
		{ID: "r1", Scope: "global", Content: "Only backend.", Roles: []string{"backend"}},
		{ID: "r2", Scope: "global", Content: "Only reviewer.", Roles: []string{"reviewer"}},
		{ID: "r3", Scope: "global", Content: "Backend or reviewer.", Roles: []string{"backend", "reviewer"}},
		{ID: "r4", Scope: "global", Content: "Generic rule.", Roles: []string{}},
	}

	// 1. Test for Backend agent
	agentBackend := &models.Agent{Role: "backend"}
	gotBackend := filterRulesForAgent(rules, agentBackend, "")
	if len(gotBackend) != 3 {
		t.Fatalf("expected 3 rules for backend agent, got %d", len(gotBackend))
	}
	// r2 (reviewer only) should be excluded
	for _, r := range gotBackend {
		if r.ID == "r2" {
			t.Fatal("r2 should not be in backend rules")
		}
	}

	// 2. Test for Reviewer agent
	agentReviewer := &models.Agent{Role: "reviewer"}
	gotReviewer := filterRulesForAgent(rules, agentReviewer, "")
	if len(gotReviewer) != 3 {
		t.Fatalf("expected 3 rules for reviewer agent, got %d", len(gotReviewer))
	}
	// r1 (backend only) should be excluded
	for _, r := range gotReviewer {
		if r.ID == "r1" {
			t.Fatal("r1 should not be in reviewer rules")
		}
	}

	// 3. Test for Coder agent (no matches on backend/reviewer specific rules)
	agentCoder := &models.Agent{Role: "coder"}
	gotCoder := filterRulesForAgent(rules, agentCoder, "")
	if len(gotCoder) != 1 {
		t.Fatalf("expected 1 rule (r4) for coder agent, got %d", len(gotCoder))
	}
	if gotCoder[0].ID != "r4" {
		t.Fatalf("expected only generic rule r4, got: %#v", gotCoder[0])
	}
}

func TestParseRuleFrontmatter(t *testing.T) {
	ruleContent := `---
id: custom-test-rule
roles:
  - backend
  - coder
---
This is the actual rule content.`

	r := &models.Rule{
		ID:      "original-id",
		Content: ruleContent,
	}

	ParseRuleFrontmatter(r)

	if r.ID != "custom-test-rule" {
		t.Errorf("expected ID to be 'custom-test-rule', got '%s'", r.ID)
	}

	if len(r.Roles) != 2 || r.Roles[0] != "backend" || r.Roles[1] != "coder" {
		t.Errorf("expected Roles [backend, coder], got %#v", r.Roles)
	}

	expectedContent := "This is the actual rule content."
	if r.Content != expectedContent {
		t.Errorf("expected Content '%s', got '%s'", expectedContent, r.Content)
	}
}

func TestFilterRulesForAgent_RunTestsHiddenFromPlanner(t *testing.T) {
	rules := []models.Rule{
		{ID: "run-tests", Scope: "global", Content: "Execute automated tests before merging.", Roles: []string{"coder", "reviewer"}},
	}
	agentPlanner := &models.Agent{Role: "planner"}
	got := filterRulesForAgent(rules, agentPlanner, "")
	if len(got) != 0 {
		t.Fatalf("expected run-tests rule to be hidden from planner, but got %d rules", len(got))
	}
}
