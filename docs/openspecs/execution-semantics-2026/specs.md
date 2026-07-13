# Specs: Execution Semantics 2026

## Added Requirements

### REQ-001: Execution IR and Prompt Compiler
> ❌ Status: Not Started

**Scenario:**
- WHEN the Planner finishes task decomposition
- THEN it must output a structured `Execution IR` instead of a natural language prompt
- AND the `Prompt Compiler` must render this IR into the final LLM prompt.

### REQ-002: Runtime State Machine and Phase Budgets
> ❌ Status: Not Started

**Scenario:**
- WHEN a node execution begins
- THEN it must progress strictly through `DISCOVERY`, `PLAN_READY`, `IMPLEMENTATION`, and `VALIDATION` states
- AND each state must enforce its own independent iteration budget and tool access restrictions.

### REQ-003: ExecutionSnapshot Recovery
> ❌ Status: Not Started

**Scenario:**
- WHEN a task exhausts its implementation budget but applies partial edits
- THEN the runtime must serialize the workspace and runtime state into an `ExecutionSnapshot`
- AND must NOT rely on `git commit` to salvage the state.

### REQ-004: Intent Resolver
> ❌ Status: Not Started

**Scenario:**
- WHEN the `Execution IR` specifies a semantic intent (e.g., "Create UserRepository")
- THEN the `Intent Resolver` must map it to concrete physical paths (e.g., `internal/repository/user.go`) before implementation begins.

## Modified Requirements

### REQ-M01: Deprecate Global Tool Loop
> ❌ Status: Not Started

**Scenario:**
- WHEN tools are executed
- THEN they must be constrained by the current active state (e.g., `create_file` is blocked during `DISCOVERY`).

## Removed Requirements
- REQ-R01: Git-based checkpointing for partial execution recovery.
