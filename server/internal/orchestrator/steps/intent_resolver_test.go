package steps

import (
	"errors"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestIntentTokens(t *testing.T) {
	cases := map[string][]string{
		"UserRepository":  {"user", "repository"},
		"user_repository": {"user", "repository"},
		"user-repository": {"user", "repository"},
		"APIClient":       {"api", "client"},
		"":                {},
	}
	for input, want := range cases {
		got := intentTokens(input)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("intentTokens(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestResolveIntent_Resolvable(t *testing.T) {
	ir := models.ExecutionIR{
		NodeID: "node_1",
		Intent: models.Intent{Capability: "UserRepository", Operation: "create"},
	}
	candidates := []models.AffectedFile{
		{File: "internal/repository/user.go", Reason: "new repository"},
		{File: "internal/repository/order.go", Reason: "unrelated"},
	}

	targets, err := ResolveIntent(ir, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 || targets[0] != "internal/repository/user.go" {
		t.Errorf("unexpected targets: %v", targets)
	}
}

func TestResolveIntent_Ambiguous(t *testing.T) {
	ir := models.ExecutionIR{
		NodeID: "node_1",
		Intent: models.Intent{Capability: "UserRepository", Operation: "modify"},
	}
	candidates := []models.AffectedFile{
		{File: "internal/repository/user.go"},
		{File: "internal/repository/user_test.go"},
		{File: "internal/repository/order.go"},
	}

	targets, err := ResolveIntent(ir, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Errorf("expected 2 ambiguous matches, got %v", targets)
	}
}

func TestResolveIntent_Unresolvable(t *testing.T) {
	ir := models.ExecutionIR{
		NodeID: "node_1",
		Intent: models.Intent{Capability: "PaymentGateway", Operation: "create"},
	}
	candidates := []models.AffectedFile{
		{File: "internal/repository/user.go"},
	}

	_, err := ResolveIntent(ir, candidates)
	if err == nil {
		t.Fatal("expected error for unresolvable intent, got nil")
	}
	var resErr *IntentResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *IntentResolutionError, got %T", err)
	}
	if resErr.NodeID != "node_1" || resErr.Capability != "PaymentGateway" {
		t.Errorf("unexpected error fields: %+v", resErr)
	}
}

func TestResolveIntent_EmptyCapability(t *testing.T) {
	ir := models.ExecutionIR{NodeID: "node_1", Intent: models.Intent{Capability: "", Operation: "create"}}
	_, err := ResolveIntent(ir, []models.AffectedFile{{File: "internal/repository/user.go"}})
	if err == nil {
		t.Fatal("expected error for empty capability, got nil")
	}
}

func TestResolveExecutionIRTargets_PartialResolution(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{
			{File: "internal/repository/user.go"},
		},
		ExecutionIRs: []models.ExecutionIR{
			{NodeID: "node_ok", Intent: models.Intent{Capability: "UserRepository", Operation: "create"}},
			{NodeID: "node_bad", Intent: models.Intent{Capability: "PaymentGateway", Operation: "create"}},
		},
	}

	resolved, err := ResolveExecutionIRTargets(analysis)
	if err == nil {
		t.Fatal("expected aggregated error for the unresolvable node, got nil")
	}
	if !strings.Contains(err.Error(), "node_bad") {
		t.Errorf("expected error to mention node_bad, got: %v", err)
	}
	if targets, ok := resolved["node_ok"]; !ok || len(targets) != 1 {
		t.Errorf("expected node_ok to resolve, got: %v", resolved)
	}
	if _, ok := resolved["node_bad"]; ok {
		t.Errorf("node_bad should not appear in resolved map, got: %v", resolved)
	}
}

func TestResolveExecutionIRTargets_AllResolved(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{{File: "internal/repository/user.go"}},
		ExecutionIRs: []models.ExecutionIR{
			{NodeID: "node_ok", Intent: models.Intent{Capability: "UserRepository", Operation: "create"}},
		},
	}
	resolved, err := ResolveExecutionIRTargets(analysis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved entry, got %v", resolved)
	}
}
