package governance

import (
	"strings"
	"testing"
)

func hasMessage(errs []ValidationError, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidateDAG_ValidGraphHasNoErrors(t *testing.T) {
	steps := []StepSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}
	if errs := ValidateDAG(steps); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateDAG_UnresolvedDependency(t *testing.T) {
	steps := []StepSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"nonexistent"}},
	}
	errs := ValidateDAG(steps)
	if !hasMessage(errs, "unresolved dependency") {
		t.Fatalf("expected unresolved dependency error, got %v", errs)
	}
}

func TestValidateDAG_CycleDetected(t *testing.T) {
	steps := []StepSpec{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	errs := ValidateDAG(steps)
	if !hasMessage(errs, "cycle detected") {
		t.Fatalf("expected cycle error, got %v", errs)
	}
}

func TestValidateDAG_MultipleEntries(t *testing.T) {
	steps := []StepSpec{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}
	errs := ValidateDAG(steps)
	if !hasMessage(errs, "multiple entry steps") {
		t.Fatalf("expected multiple entry error, got %v", errs)
	}
}

func TestValidateDAG_UnreachableStep(t *testing.T) {
	steps := []StepSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "orphan", DependsOn: []string{"orphan_base"}},
		{ID: "orphan_base"},
	}
	errs := ValidateDAG(steps)
	if !hasMessage(errs, "multiple entry steps") {
		t.Fatalf("expected multiple entry error (two disconnected components), got %v", errs)
	}
}

// checkNoDeadEnds is exercised directly (rather than through ValidateDAG)
// because in a graph that has already passed the acyclic + single-entry +
// reachability checks, every node is, by construction, guaranteed a forward
// path to some leaf/terminal — a true dead end can't arise there. The unit
// still guards the function's own logic in isolation (e.g. if it's ever
// reused before those upstream checks, or terminals are supplied
// explicitly rather than inferred).
func TestCheckNoDeadEnds_ReportsNodeNotReachingTerminal(t *testing.T) {
	steps := []StepSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
	}
	byID := map[string]StepSpec{"a": steps[0], "b": steps[1]}
	// Deliberately declare only "a" as terminal (as if config supplied an
	// explicit incomplete terminal set) so "b" has no path to it.
	errs := checkNoDeadEnds(steps, byID, map[string]bool{"a": true})
	if !hasMessage(errs, "dead-end step: b") {
		t.Fatalf("expected dead-end error for 'b', got %v", errs)
	}
}

func TestValidateDAG_EmptyReturnsNoErrors(t *testing.T) {
	if errs := ValidateDAG(nil); len(errs) != 0 {
		t.Fatalf("expected no errors for empty graph, got %v", errs)
	}
}
