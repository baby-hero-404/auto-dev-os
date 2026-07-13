# Proposal: Execution Semantics 2026

## Why
The current orchestration engine relies heavily on a natural-language, prompt-driven continuous reasoning loop (a monolithic `RunToolLoop`). Downstream agents repeatedly re-analyze the repository, leading to semantic drift, massive token consumption, and resource contention (global iteration budgets are exhausted during discovery before implementation begins). Furthermore, partial successes fail to salvage due to a strict dependency on Git identity.

## What Changes

### Issue 1: Monolithic Tool Loop
- Dismantle the global `RunToolLoop`.
- Introduce a Runtime State Machine with explicit phases (`DISCOVERY`, `PLAN_READY`, `IMPLEMENTATION`, `VALIDATION`) and phase-specific iteration budgets.

### Issue 2: Natural Language Semantic Drift
- Deprecate prompt/JSON outputs from Planner agents.
- Introduce `Execution IR` as a strict semantic execution contract.
- Introduce `Prompt Compiler` to render `Execution IR` into provider-specific prompts.

### Issue 3: Tight Coupling to Physical Repository
- Introduce `Intent Resolver` to translate semantic planner intents into physical file paths and operations.

### Issue 4: Fragile Checkpointing
- Replace Git-based recovery with `ExecutionSnapshot` for pure state-based checkpointing and replayability.

## Capabilities

### New Capabilities
- Deterministic state transitions (Runtime State Machine).
- Provider-agnostic prompt rendering (Prompt Compiler).
- Non-Git execution checkpointing (ExecutionSnapshot).
- Abstract semantic targeting (Intent Resolver).

### Modified Capabilities
- `RunToolLoop` replaced by phase-specific state machine execution.

### Removed Capabilities
- Git-dependent salvage mechanism.

## Impact

| Area | Files Affected |
|------|----------------|
| Orchestrator | `server/internal/orchestrator/llmrunner/toolloop.go` |
| Planner | `server/internal/orchestrator/steps/analyze.go` |
| Prompts | `server/internal/prompts/builder.go` |
| State | `server/internal/sandbox/git.go`, `server/internal/orchestrator/llm_step.go` |
