package llmrunner

import (
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// NodeState is a phase in the per-node execution state machine
// (docs/openspecs/execution-semantics-2026/design.md § Node State Machine).
type NodeState string

const (
	StateDiscovery      NodeState = "DISCOVERY"
	StatePlanReady      NodeState = "PLAN_READY"
	StateImplementation NodeState = "IMPLEMENTATION"
	StateValidation     NodeState = "VALIDATION"
	StateDone           NodeState = "DONE"
	StateSalvaged       NodeState = "SALVAGED"
	StateFailed         NodeState = "FAILED"
)

// Terminal reports whether s is one of the machine's terminal states.
func (s NodeState) Terminal() bool {
	switch s {
	case StateDone, StateSalvaged, StateFailed:
		return true
	default:
		return false
	}
}

// PhaseViolationError is returned when a tool call is attempted outside its
// current state's allowlist (REQ-M01). It is meant to be surfaced to the
// model as corrective tool-result feedback, not to abort the node — the
// caller decides whether/how to feed this back into the loop.
type PhaseViolationError struct {
	State NodeState
	Tool  string
}

func (e *PhaseViolationError) Error() string {
	return fmt.Sprintf("tool %q is not permitted during %s", e.Tool, e.State)
}

// readOnlyTools are the tools registered in internal/tool/tools/ that only
// inspect the workspace (no mutation). Shared across every state that grants
// read access.
var readOnlyTools = []string{
	"read_file", "list_files", "grep_search", "find_symbol",
	"file_exists", "read_spec", "read_affected_files",
}

// defaultToolAllowlists implements the per-state tool allowlist column of
// design.md's transition-rules table, using the tool names actually
// registered in internal/tool/tools/. PLAN_READY is a deterministic gate
// with no LLM turn, so it allows nothing. Terminal states likewise allow
// nothing (the node is done issuing tool calls).
var defaultToolAllowlists = map[NodeState]map[string]bool{
	StateDiscovery:      toolSet(readOnlyTools...),
	StateImplementation: toolSet(append(append([]string{}, readOnlyTools...), "search_replace", "create_file")...),
	StateValidation:     toolSet(append(append([]string{}, readOnlyTools...), "verify_workspace")...),
}

func toolSet(tools ...string) map[string]bool {
	m := make(map[string]bool, len(tools))
	for _, t := range tools {
		m[t] = true
	}
	return m
}

// StateMachine drives one execution node through
// DISCOVERY -> PLAN_READY -> IMPLEMENTATION -> VALIDATION, with terminal
// DONE / SALVAGED / FAILED states (REQ-002). It owns per-phase iteration
// budgets and tool allowlists but performs no I/O itself — it does not call
// the LLM or execute tools. A caller drives it by reporting the outcome of
// each iteration via the Advise*/Resolve* methods; wiring this into the
// actual execution loop is Task 2.2 (flag-gated migration off RunToolLoop).
type StateMachine struct {
	current    NodeState
	budget     models.PhaseBudgets
	used       map[NodeState]int
	allowlists map[NodeState]map[string]bool
	hasEdits   bool // whether IMPLEMENTATION has ever applied an edit this node's lifetime
}

// NewStateMachine starts a machine in DISCOVERY with the given phase
// budgets (typically ExecutionIR.Budget).
func NewStateMachine(budget models.PhaseBudgets) *StateMachine {
	return NewStateMachineFrom(StateDiscovery, budget)
}

// NewStateMachineFrom starts a machine in the given initial state with the
// provided phase budgets. Use this when the calling step already has full
// context (e.g. fix step after review) and the DISCOVERY phase can be skipped.
func NewStateMachineFrom(initial NodeState, budget models.PhaseBudgets) *StateMachine {
	return &StateMachine{
		current:    initial,
		budget:     budget,
		used:       make(map[NodeState]int),
		allowlists: defaultToolAllowlists,
	}
}

// Current returns the machine's current state.
func (sm *StateMachine) Current() NodeState { return sm.current }

// budgetFor returns the configured iteration budget for an LLM-driven state.
// PLAN_READY and terminal states have no budget — they consume none
// (design.md transition-rules table).
func (sm *StateMachine) budgetFor(s NodeState) int {
	switch s {
	case StateDiscovery:
		return sm.budget.Discovery
	case StateImplementation:
		return sm.budget.Implementation
	case StateValidation:
		return sm.budget.Validation
	default:
		return 0
	}
}

// Remaining returns the number of iterations left in the current state's
// budget (0 for PLAN_READY and terminal states).
func (sm *StateMachine) Remaining() int {
	remaining := sm.budgetFor(sm.current) - sm.used[sm.current]
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ToolAllowed reports whether tool is permitted in the current state.
func (sm *StateMachine) ToolAllowed(tool string) bool {
	allowed, ok := sm.allowlists[sm.current]
	return ok && allowed[tool]
}

// CheckTool returns a *PhaseViolationError if tool is not permitted in the
// current state, nil otherwise (REQ-M01).
func (sm *StateMachine) CheckTool(tool string) error {
	if sm.ToolAllowed(tool) {
		return nil
	}
	return &PhaseViolationError{State: sm.current, Tool: tool}
}

func (sm *StateMachine) wrongStateErr(method string, want NodeState) error {
	return fmt.Errorf("state machine: %s called while in %s, not %s", method, sm.current, want)
}

// AdviseDiscovery records one DISCOVERY iteration. If contextSufficient is
// true, transitions to PLAN_READY. Otherwise, if the discovery budget is now
// exhausted, transitions to FAILED. If neither, the machine stays in
// DISCOVERY for another iteration.
func (sm *StateMachine) AdviseDiscovery(contextSufficient bool) (NodeState, error) {
	if sm.current != StateDiscovery {
		return sm.current, sm.wrongStateErr("AdviseDiscovery", StateDiscovery)
	}
	sm.used[StateDiscovery]++

	if contextSufficient {
		sm.current = StatePlanReady
		return sm.current, nil
	}
	if sm.used[StateDiscovery] >= sm.budget.Discovery {
		sm.current = StateFailed
	}
	return sm.current, nil
}

// ResolvePlan is the deterministic PLAN_READY gate: no LLM turn, no budget
// consumed. intentsResolved should be true only when every ExecutionIR
// intent for this node resolved to physical targets (REQ-004). Transitions
// to IMPLEMENTATION or FAILED.
func (sm *StateMachine) ResolvePlan(intentsResolved bool) (NodeState, error) {
	if sm.current != StatePlanReady {
		return sm.current, sm.wrongStateErr("ResolvePlan", StatePlanReady)
	}
	if intentsResolved {
		sm.current = StateImplementation
	} else {
		sm.current = StateFailed
	}
	return sm.current, nil
}

// AdviseImplementation records one IMPLEMENTATION iteration. editApplied
// marks whether this particular iteration produced a workspace edit
// (tracked cumulatively for the SALVAGED-vs-FAILED distinction below). If
// editsComplete is true, transitions to VALIDATION. Otherwise, if the
// implementation budget is now exhausted, transitions to SALVAGED when at
// least one edit was ever applied this node's lifetime, or FAILED when
// none was — there is nothing to salvage from a node that never touched the
// workspace, so a snapshot would be pointless (an extension beyond
// design.md's diagram, which assumes edits exist by the time budget runs
// out).
func (sm *StateMachine) AdviseImplementation(editsComplete bool, editApplied bool) (NodeState, error) {
	if sm.current != StateImplementation {
		return sm.current, sm.wrongStateErr("AdviseImplementation", StateImplementation)
	}
	sm.used[StateImplementation]++
	if editApplied {
		sm.hasEdits = true
	}

	if editsComplete {
		sm.current = StateValidation
		return sm.current, nil
	}
	if sm.used[StateImplementation] >= sm.budget.Implementation {
		if sm.hasEdits {
			sm.current = StateSalvaged
		} else {
			sm.current = StateFailed
		}
	}
	return sm.current, nil
}

// AdviseValidation records one VALIDATION iteration. If checksPassed is
// true, transitions to DONE. Otherwise it loops back to IMPLEMENTATION only
// while the IMPLEMENTATION budget still has room AND VALIDATION's own
// budget (tracked cumulatively across every visit to VALIDATION for this
// node, not per-visit) hasn't itself run out; otherwise it goes to SALVAGED
// (or FAILED if IMPLEMENTATION never actually applied an edit).
func (sm *StateMachine) AdviseValidation(checksPassed bool) (NodeState, error) {
	if sm.current != StateValidation {
		return sm.current, sm.wrongStateErr("AdviseValidation", StateValidation)
	}
	sm.used[StateValidation]++

	if checksPassed {
		sm.current = StateDone
		return sm.current, nil
	}

	validationExhausted := sm.used[StateValidation] >= sm.budget.Validation
	implementationBudgetRemains := sm.used[StateImplementation] < sm.budget.Implementation
	if !validationExhausted && implementationBudgetRemains {
		sm.current = StateImplementation
		return sm.current, nil
	}

	if sm.hasEdits {
		sm.current = StateSalvaged
	} else {
		sm.current = StateFailed
	}
	return sm.current, nil
}
