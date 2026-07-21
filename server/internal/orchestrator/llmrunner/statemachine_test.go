package llmrunner

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func budgets(discovery, implementation, validation int) models.PhaseBudgets {
	return models.PhaseBudgets{Discovery: discovery, Implementation: implementation, Validation: validation}
}

// --- DISCOVERY ---

func TestStateMachine_Discovery_ContextSufficient(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	state, err := sm.AdviseDiscovery(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StatePlanReady {
		t.Errorf("expected PLAN_READY, got %s", state)
	}
}

func TestStateMachine_Discovery_BudgetExhausted_TransitionsFailed(t *testing.T) {
	sm := NewStateMachine(budgets(2, 3, 3))
	if state, _ := sm.AdviseDiscovery(false); state != StateDiscovery {
		t.Fatalf("expected to stay in DISCOVERY after iteration 1, got %s", state)
	}
	state, err := sm.AdviseDiscovery(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateFailed {
		t.Errorf("expected FAILED after discovery budget exhausted, got %s", state)
	}
	if !state.Terminal() {
		t.Error("expected FAILED to be terminal")
	}
}

func TestStateMachine_Discovery_WrongStateCall(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	sm.current = StateImplementation
	if _, err := sm.AdviseDiscovery(true); err == nil {
		t.Fatal("expected error calling AdviseDiscovery outside DISCOVERY")
	}
}

// --- PLAN_READY ---

func TestStateMachine_PlanReady_IntentsResolved(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	sm.AdviseDiscovery(true)
	if sm.Remaining() != 0 {
		t.Errorf("PLAN_READY should consume no budget, Remaining()=%d", sm.Remaining())
	}
	state, err := sm.ResolvePlan(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateImplementation {
		t.Errorf("expected IMPLEMENTATION, got %s", state)
	}
}

func TestStateMachine_PlanReady_UnresolvableIntent(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	sm.AdviseDiscovery(true)
	state, err := sm.ResolvePlan(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateFailed {
		t.Errorf("expected FAILED, got %s", state)
	}
}

func TestStateMachine_PlanReady_ConsumesNoBudget(t *testing.T) {
	sm := NewStateMachine(budgets(0, 3, 3))
	sm.AdviseDiscovery(true) // discovery budget is 0, but contextSufficient=true bypasses exhaustion check
	if sm.Current() != StatePlanReady {
		t.Fatalf("expected PLAN_READY, got %s", sm.Current())
	}
	sm.ResolvePlan(true)
	if sm.Current() != StateImplementation {
		t.Fatalf("expected IMPLEMENTATION, got %s", sm.Current())
	}
}

// --- IMPLEMENTATION ---

func advanceToImplementation(sm *StateMachine) {
	sm.AdviseDiscovery(true)
	sm.ResolvePlan(true)
}

func TestStateMachine_Implementation_EditsComplete(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	advanceToImplementation(sm)
	state, err := sm.AdviseImplementation(true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateValidation {
		t.Errorf("expected VALIDATION, got %s", state)
	}
}

func TestStateMachine_Implementation_BudgetExhausted_WithEdits_Salvaged(t *testing.T) {
	sm := NewStateMachine(budgets(3, 2, 3))
	advanceToImplementation(sm)
	sm.AdviseImplementation(false, true) // iteration 1: edit applied, not complete
	state, err := sm.AdviseImplementation(false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateSalvaged {
		t.Errorf("expected SALVAGED (edits exist), got %s", state)
	}
}

func TestStateMachine_Implementation_BudgetExhausted_NoEdits_Failed(t *testing.T) {
	sm := NewStateMachine(budgets(3, 2, 3))
	advanceToImplementation(sm)
	sm.AdviseImplementation(false, false)
	state, err := sm.AdviseImplementation(false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateFailed {
		t.Errorf("expected FAILED (no edits ever applied), got %s", state)
	}
}

func TestStateMachine_Implementation_WrongStateCall(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	if _, err := sm.AdviseImplementation(true, true); err == nil {
		t.Fatal("expected error calling AdviseImplementation outside IMPLEMENTATION")
	}
}

// --- VALIDATION ---

func advanceToValidation(sm *StateMachine) {
	advanceToImplementation(sm)
	sm.AdviseImplementation(true, true)
}

func TestStateMachine_Validation_ChecksPass(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	advanceToValidation(sm)
	state, err := sm.AdviseValidation(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateDone {
		t.Errorf("expected DONE, got %s", state)
	}
	if !state.Terminal() {
		t.Error("expected DONE to be terminal")
	}
}

func TestStateMachine_Validation_ChecksFail_LoopsBackWhileBudgetRemains(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	advanceToValidation(sm) // 1 implementation iteration used, 2 remain
	state, err := sm.AdviseValidation(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateImplementation {
		t.Errorf("expected IMPLEMENTATION (budget remains), got %s", state)
	}
}

func TestStateMachine_Validation_ChecksFail_ImplementationBudgetExhausted_Salvaged(t *testing.T) {
	sm := NewStateMachine(budgets(3, 1, 3))
	advanceToValidation(sm) // consumes the sole implementation iteration
	state, err := sm.AdviseValidation(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateSalvaged {
		t.Errorf("expected SALVAGED (implementation budget exhausted, edits exist), got %s", state)
	}
}

func TestStateMachine_Validation_OwnBudgetExhausted_Salvaged(t *testing.T) {
	sm := NewStateMachine(budgets(3, 5, 1))
	advanceToValidation(sm)
	state, err := sm.AdviseValidation(false) // consumes the sole validation iteration
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateSalvaged {
		t.Errorf("expected SALVAGED (validation's own budget exhausted despite implementation budget remaining), got %s", state)
	}
}

func TestStateMachine_Validation_WrongStateCall(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	if _, err := sm.AdviseValidation(true); err == nil {
		t.Fatal("expected error calling AdviseValidation outside VALIDATION")
	}
}

// --- Tool allowlists (REQ-M01) ---

func TestStateMachine_ToolAllowlist_Discovery(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	if !sm.ToolAllowed("read_file") {
		t.Error("expected read_file allowed in DISCOVERY")
	}
	if sm.ToolAllowed("create_file") {
		t.Error("expected create_file denied in DISCOVERY")
	}
	if err := sm.CheckTool("create_file"); err == nil {
		t.Fatal("expected PhaseViolationError for create_file in DISCOVERY")
	} else if _, ok := err.(*PhaseViolationError); !ok {
		t.Errorf("expected *PhaseViolationError, got %T", err)
	}
}

func TestStateMachine_ToolAllowlist_Implementation(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	advanceToImplementation(sm)
	if !sm.ToolAllowed("search_replace") || !sm.ToolAllowed("create_file") {
		t.Error("expected write tools allowed in IMPLEMENTATION")
	}
	if !sm.ToolAllowed("read_file") {
		t.Error("expected read_file still allowed in IMPLEMENTATION")
	}
	if sm.ToolAllowed("verify_workspace") {
		t.Error("expected verify_workspace denied in IMPLEMENTATION")
	}
}

func TestStateMachine_ToolAllowlist_Validation(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	advanceToValidation(sm)
	if !sm.ToolAllowed("verify_workspace") {
		t.Error("expected verify_workspace allowed in VALIDATION")
	}
	if sm.ToolAllowed("create_file") {
		t.Error("expected create_file denied in VALIDATION")
	}
}

func TestStateMachine_ToolAllowlist_PlanReadyAllowsNothing(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	sm.AdviseDiscovery(true)
	if sm.ToolAllowed("read_file") {
		t.Error("expected no tools allowed in PLAN_READY (deterministic gate, no LLM turn)")
	}
}

func TestStateMachine_ToolAllowlist_TerminalStatesAllowNothing(t *testing.T) {
	sm := NewStateMachine(budgets(3, 3, 3))
	advanceToValidation(sm)
	sm.AdviseValidation(true) // -> DONE
	if sm.ToolAllowed("read_file") {
		t.Error("expected no tools allowed once terminal")
	}
}

// --- Remaining() bookkeeping ---

func TestStateMachine_Remaining(t *testing.T) {
	sm := NewStateMachine(budgets(3, 5, 2))
	if sm.Remaining() != 3 {
		t.Errorf("expected 3 remaining in fresh DISCOVERY, got %d", sm.Remaining())
	}
	sm.AdviseDiscovery(false)
	if sm.Remaining() != 2 {
		t.Errorf("expected 2 remaining after 1 iteration, got %d", sm.Remaining())
	}
}
