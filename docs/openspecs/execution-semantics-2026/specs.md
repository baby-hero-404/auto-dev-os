# Specs: Execution Semantics 2026

## Added Requirements

### REQ-001: Execution IR and Prompt Compiler
> ⚠️ Status: In Progress — struct/schema/compiler/validation done (server/pkg/models/ir.go, internal/prompts/compiler.go); deterministic fallback wired in plan.go; direct LLM emission in analyze.go in-flight and unverified (see tasks.md Task 1.1 note)

**Scenario: Planner emits IR instead of prose**
- WHEN the Planner finishes task decomposition
- THEN it must output a structured `Execution IR` instead of a natural-language prompt
- AND the `Prompt Compiler` must be the only component that renders the IR into a final LLM prompt.

**Scenario: Invalid IR is rejected before execution**
- WHEN Planner output fails `Execution IR` JSON Schema validation
- THEN the runtime must reject it before any prompt is compiled or tool is executed
- AND surface a structured validation error naming the failing fields.

### REQ-002: Runtime State Machine and Phase Budgets
> ⚠️ Status: In Progress — FSM implemented and tested (internal/orchestrator/llmrunner/statemachine.go); not yet driving real execution (Task 2.2)

**Scenario: Phased execution**
- WHEN a node execution begins
- THEN it must progress strictly through `DISCOVERY` → `PLAN_READY` → `IMPLEMENTATION` → `VALIDATION`
- AND each LLM-driven state (`DISCOVERY`, `IMPLEMENTATION`, `VALIDATION`) must enforce its own independent iteration budget and tool allowlist
- AND `PLAN_READY` is a transient, non-LLM gate (intent resolution + budget check) with no iteration budget.

**Scenario: Validation failure loops back bounded**
- WHEN `VALIDATION` fails (tests or acceptance checks)
- THEN the machine must transition back to `IMPLEMENTATION` with the failure context
- AND only while the `IMPLEMENTATION` budget is not exhausted; otherwise transition to `SALVAGED` (snapshot taken, REQ-003).

### REQ-003: ExecutionSnapshot Recovery
> ❌ Status: Not Started

**Scenario: Salvage without Git**
- WHEN a node exhausts its `IMPLEMENTATION` budget after applying partial edits
- THEN the runtime must serialize workspace diff and runtime state into an `ExecutionSnapshot`
- AND must NOT invoke `git commit` (or require Git identity) to salvage the state.

**Scenario: Replay from snapshot**
- WHEN the orchestrator resumes a node from an `ExecutionSnapshot`
- THEN the restored workspace and state machine position must be byte-identical to the snapshot contents
- AND execution continues from the recorded state with the remaining budget.

### REQ-004: Intent Resolver
> ⚠️ Status: In Progress — resolution logic done (steps/intent_resolver.go); fail-fast enforcement deferred to Task 2.1, see tasks.md Task 1.2 note

**Scenario: Semantic intent resolves to physical targets**
- WHEN the `Execution IR` specifies a semantic intent (e.g., "Create UserRepository")
- THEN the `Intent Resolver` must map it to concrete physical paths (e.g., `internal/repository/user.go`) during `PLAN_READY`, before `IMPLEMENTATION` begins
- AND the resolved targets define the write scope for `IMPLEMENTATION` (see design.md § Security).

**Scenario: Unresolvable intent fails fast**
- WHEN an intent cannot be mapped to any physical target
- THEN the node must fail at `PLAN_READY` with a resolution error
- AND must NOT enter `IMPLEMENTATION` with an unscoped write permission.

## Modified Requirements

### REQ-M01: Phase-Scoped Tool Access (replaces global tool loop)
> ⚠️ Status: In Progress — allowlist enforcement implemented in the FSM (StateMachine.CheckTool); RunToolLoop itself doesn't consult it yet (Task 2.2)

**Scenario:**
- WHEN a tool call is requested
- THEN it must be checked against the allowlist of the current active state (e.g., `create_file` is blocked during `DISCOVERY`; read tools remain available in all LLM states)
- AND a denied call returns a structured phase-violation error to the model rather than terminating the node.

## Removed Requirements
- REQ-R01: Git-based checkpointing for partial execution recovery (salvage-commit path in `patch_retry_loop.go`). Git remains for terminal PR export only.
