package steps

import (
	"errors"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestIntentTokens(t *testing.T) {
	cases := map[string][]string{
		"UserRepository":      {"user", "repository"},
		"user_repository":     {"user", "repository"},
		"user-repository":     {"user", "repository"},
		"APIClient":           {"api", "client"},
		"SyncEngineScheduler": {"sync", "engine", "scheduler"},
		"":                    {},
		"Thiết lập cấu trúc dự án và SQLite": {}, // Natural language / Vietnamese should be skipped
		"This is a sentence to skip":         {}, // Natural language / English sentence >= 3 words should be skipped
	}
	for input, want := range cases {
		got := intentTokens(input)
		if len(want) == 0 && len(got) == 0 {
			continue
		}
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

	targets, dropped, err := ResolveIntent(ir, candidates, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dropped) != 0 {
		t.Errorf("expected 0 dropped files, got: %v", dropped)
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

	targets, _, err := ResolveIntent(ir, candidates, nil)
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

	_, _, err := ResolveIntent(ir, candidates, nil)
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
	// Verify double failure names both strategies
	if !strings.Contains(resErr.Reason, "attempted unit target_files") || !strings.Contains(resErr.Reason, "token matching fallback") {
		t.Errorf("expected error reason to name both strategies, got: %s", resErr.Reason)
	}
}

func TestResolveIntent_EmptyCapability(t *testing.T) {
	ir := models.ExecutionIR{NodeID: "node_1", Intent: models.Intent{Capability: "", Operation: "create"}}
	_, _, err := ResolveIntent(ir, []models.AffectedFile{{File: "internal/repository/user.go"}}, nil)
	if err == nil {
		t.Fatal("expected error for empty capability, got nil")
	}
}

func TestResolveIntent_Corroboration(t *testing.T) {
	ir := models.ExecutionIR{
		NodeID: "node_1",
		Intent: models.Intent{Capability: "PaymentGateway", Operation: "create"},
	}
	candidates := []models.AffectedFile{
		{File: "internal/gateway/payment.go"},
		{File: "internal/repository/user.go"},
	}
	targetFiles := []string{"internal/gateway/payment.go", "internal/gateway/uncorroborated.go"}

	targets, dropped, err := ResolveIntent(ir, candidates, targetFiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 || targets[0] != "internal/gateway/payment.go" {
		t.Errorf("expected to resolve only corroborated target file, got: %v", targets)
	}
	if len(dropped) != 1 || dropped[0] != "internal/gateway/uncorroborated.go" {
		t.Errorf("expected uncorroborated to be dropped, got: %v", dropped)
	}
}

func TestResolveIntent_VietnameseNaturalLanguage(t *testing.T) {
	ir := models.ExecutionIR{
		NodeID: "node_1",
		Intent: models.Intent{Capability: "Thiết lập cấu trúc dự án và SQLite", Operation: "create"},
	}
	candidates := []models.AffectedFile{
		{File: "server/main.go"},
		{File: "server/db.go"},
	}
	targetFiles := []string{"server/main.go", "server/db.go"}

	targets, dropped, err := ResolveIntent(ir, candidates, targetFiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dropped) != 0 {
		t.Errorf("expected 0 dropped, got %v", dropped)
	}
	if len(targets) != 2 || targets[0] != "server/main.go" || targets[1] != "server/db.go" {
		t.Errorf("expected to resolve natural language capability using corroborated targetFiles, got: %v", targets)
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

	resolved, dropped, err := ResolveExecutionIRTargets(analysis)
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
	_ = dropped
}

func TestResolveExecutionIRTargets_AllResolved(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{{File: "internal/repository/user.go"}},
		ExecutionIRs: []models.ExecutionIR{
			{NodeID: "node_ok", Intent: models.Intent{Capability: "UserRepository", Operation: "create"}},
		},
	}
	resolved, dropped, err := ResolveExecutionIRTargets(analysis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved entry, got %v", resolved)
	}
	if len(dropped) != 0 {
		t.Errorf("expected 0 dropped entry, got %v", dropped)
	}
}

func TestResolveExecutionIRTargets_MaxFiles(t *testing.T) {
	analysis := models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{
			{File: "internal/repository/user.go"},
			{File: "internal/repository/user_test.go"},
		},
		ExecutionUnits: []models.ExecutionUnit{
			{
				ID: "node_ok",
				Constraints: models.ExecutionConstraints{
					MaxFiles: 1,
				},
			},
		},
		ExecutionIRs: []models.ExecutionIR{
			{NodeID: "node_ok", Intent: models.Intent{Capability: "UserRepository", Operation: "create"}},
		},
	}

	_, _, err := ResolveExecutionIRTargets(analysis)
	if err == nil {
		t.Fatal("expected error due to MaxFiles limit exceeded, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds MaxFiles") {
		t.Errorf("expected error to mention MaxFiles limit, got: %v", err)
	}
}
