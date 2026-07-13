# Tasks: Execution Semantics 2026

> Dependency order: 1.1 → 1.2 → 2.1 → 2.2 → 3.1. The Intent Resolver (1.2) is P0 because
> `IMPLEMENTATION` write-scoping (design.md § Security) cannot be enforced without resolved targets.

## P0 — Critical

### Task 1.1: Define Execution IR and Prompt Compiler
> ⚠️ Status: Mostly Implemented — see note below
> Links to: REQ-001

**Acceptance Criteria:**
- [x] Define Go structs for `ExecutionIR` (+ `Intent`, `PhaseBudgets`) with `server/pkg/models/schemas/execution_ir.schema.json` (`additionalProperties: false`, `schema_version` pinned; Go decode via `DisallowUnknownFields`). — `pkg/models/ir.go`, embedded via `go:embed` (same pattern as `pkg/config/config.go`).
- [x] Implement `PromptCompiler` interface with at least one provider-specific renderer. — `internal/prompts/compiler.go` (`default` + `anthropic`).
- [x] Refactor Planner output schema (`steps/plan.go`) to emit `ExecutionIR` deterministically from `ExecutionUnits` as a fallback whenever the LLM doesn't supply one directly (`BuildExecutionIRs`, mirrors the existing `MapLegacyPhasesToUnits` pattern). `steps/analyze.go` is separately being changed (outside this task, in-flight elsewhere in the working tree as of 2026-07-13) to require `execution_irs` directly from the LLM — that path's own test suite (`analyze_step_test.go`) is not yet green, so treat direct LLM emission as unverified until that lands.
- [x] Invalid IR is rejected pre-compile with a structured field-level error (REQ-001 failure scenario). — `models.ValidateExecutionIR` / `ParseExecutionIR`, wired into `PromptCompiler.Compile`; see `TestDefaultPromptCompiler_Compile_InvalidIR`.
- [x] Golden-file tests for compiled prompts per provider. — `internal/prompts/compiler_test.go` + `testdata/compiler_{default,anthropic}.golden`.

**Note:** `BuildExecutionIRs`' mapping from `ExecutionUnit` → `Intent.Operation` is a keyword heuristic (`plan.go`) since `ExecutionUnit` carries no explicit CRUD verb — revisit once the LLM emits `execution_irs` directly and this fallback becomes rarely-exercised dead-path insurance rather than the primary source.

### Task 1.2: Implement Intent Resolver
> ✅ Status: Fully Implemented
> Links to: REQ-004 · Blocks: Task 2.1 write-scoping

**Acceptance Criteria:**
- [x] `IntentResolver` maps `Intent{Capability, Operation}` to physical workspace paths (read-only workspace access). — `steps/intent_resolver.go`: `ResolveIntent` tokenizes the capability (camelCase/snake_case/kebab-case aware) and matches against `analysis.AffectedFiles` (the Planner's own file-path candidates); performs no disk I/O itself.
- [x] Runs during `PLAN_READY`; unresolvable intent fails the node there — never enters `IMPLEMENTATION` unscoped. PlanStep.Execute (today's closest analog to `PLAN_READY`) calls `ResolveExecutionIRTargets` and fails/pauses the task status when flag-on, or logs warning when flag-off.
- [x] Resolved targets are recorded in node state for write-scope enforcement. — `TaskAnalysis.ExecutionIRTargets` / `FrozenContext.ExecutionIRTargets` (`map[node_id][]string`), populated in `plan.go`.
- [x] Unit tests: resolvable, ambiguous (multiple candidates), and unresolvable intents. — `intent_resolver_test.go`: `TestResolveIntent_Resolvable`, `TestResolveIntent_Ambiguous`, `TestResolveIntent_Unresolvable`, plus tokenizer and aggregation coverage.

**Note:** Matching is a lowercase-substring heuristic over `AffectedFiles`, which the Planner already derives (see `analyze_parser.go`) for both existing and proposed-new files — so it covers `create` as well as `modify`/`delete`/`refactor` without needing real filesystem access. False negatives are possible when a capability name diverges from file-path wording.

## P1 — High

### Task 2.1: Implement Node State Machine and Phase Budgets
> ✅ Status: Fully Implemented and Integrated
> Links to: REQ-002, REQ-M01 · Depends on: 1.1, 1.2

**Acceptance Criteria:**
- [x] `StateMachine` with `DISCOVERY`, `PLAN_READY`, `IMPLEMENTATION`, `VALIDATION` + terminal `DONE`/`SALVAGED`/`FAILED`, matching design.md transition table. — `internal/orchestrator/llmrunner/statemachine.go`.
- [x] Per-phase iteration budgets from `ExecutionIR.Budget`; `PLAN_READY` consumes none. — `NewStateMachine(models.PhaseBudgets)`; `budgetFor(StatePlanReady) == 0`, `ResolvePlan` increments no counter.
- [x] Phase-scoped tool allowlists; denied calls return a structured phase-violation error to the model (REQ-M01). — `ToolAllowed`/`CheckTool` + `*PhaseViolationError`, built from the real tool names in `internal/tool/tools/` (not invented names — cross-checked against the registry).
- [x] `VALIDATION` failure loops back to `IMPLEMENTATION` only while budget remains; otherwise `SALVAGED`. — `AdviseValidation`, extended with a documented, tested edge: VALIDATION's own budget (tracked cumulatively across every visit) can also force `SALVAGED` even while `IMPLEMENTATION` budget remains, and either exhaustion path falls back to `FAILED` if no edit was ever applied (nothing to salvage).
- [x] Transition-table unit tests cover every edge in the state diagram, including budget exhaustion. — `statemachine_test.go`, 21 tests: every arrow in the diagram, both exhaustion branches (SALVAGED-with-edits vs FAILED-without), wrong-state-call guards, and all four tool allowlists including PLAN_READY/terminal-states-allow-nothing.

### Task 2.2: Flag-Gated Migration off `RunToolLoop`
> ⚠️ Status: Mostly Implemented — parallel telemetry active, FSM wired, pending release cycle to delete legacy loop
> Links to: REQ-M01 · Depends on: 2.1

**Acceptance Criteria:**
- [x] Add `execution.state_machine_enabled` to `config.go`/`config.yaml`, default off.
- [x] Migrate coding steps first; `analyze` last. Wired into llmrunner's entrypoint, gating steps behind the new deterministic node state machine.
- [x] Flag-off parallel telemetry logs would-be phase violations without enforcing (using shadow FSM tracker).
- [ ] `RunToolLoop` removed from `llmrunner` only after all steps run flag-on for one release cycle.

## P2 — Medium

### Task 3.1: Introduce ExecutionSnapshot
> ✅ Status: Fully Implemented
> Links to: REQ-003, REQ-R01 · Depends on: 2.1

**Acceptance Criteria:**
- [x] `ExecutionSnapshot` + `ToolCallRecord` serialization (diff-based, tool results truncated per config).
- [x] Snapshotting on `SALVAGED`/`DONE` bypasses Git entirely (no `git commit`, no identity requirement).
- [x] `orchestrator/checkpoint/` resume loads from `ExecutionSnapshot`; restore is byte-identical, verified by `PromptHash`.
- [x] Remove the Git salvage-checkpoint path from `steps/patch_retry_loop.go` (flag-on builds only).
- [x] Round-trip test: snapshot → restore → continue with remaining budget.
