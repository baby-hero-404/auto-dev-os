# Tasks: Execution Semantics 2026

## P0 — Critical

### Task 1.1: Define Execution IR and Prompt Compiler
> Links to: REQ-001

**Acceptance Criteria:**
- [ ] Define Go structs for `ExecutionIR`.
- [ ] Implement `PromptCompiler` interface with at least one provider-specific rendering logic.
- [ ] Refactor Planner output schema to emit `ExecutionIR`.

## P1 — High

### Task 2.1: Implement Node State Machine and Phase Budgets
> Links to: REQ-002, REQ-M01

**Acceptance Criteria:**
- [ ] Remove monolithic `RunToolLoop` from `llmrunner`.
- [ ] Implement `StateMachine` with `DISCOVERY`, `PLAN_READY`, `IMPLEMENTATION`, and `VALIDATION` states.
- [ ] Enforce phase-specific tool availability (e.g., no edit tools during `DISCOVERY`).

## P2 — Medium

### Task 3.1: Introduce ExecutionSnapshot
> Links to: REQ-003, REQ-R01

**Acceptance Criteria:**
- [ ] Define `ExecutionSnapshot` serialization logic.
- [ ] Implement workspace snapshotting (e.g., copy files or store raw diffs) bypassing Git.
- [ ] Update orchestrator resume logic to load from `ExecutionSnapshot`.

## P3 — Low

### Task 4.1: Add Intent Resolver
> Links to: REQ-004

**Acceptance Criteria:**
- [ ] Implement `IntentResolver` to map semantic operations to physical workspace files.
- [ ] Integrate resolver step between `Execution IR` emission and `DISCOVERY` state.
