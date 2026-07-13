# Proposal: Execution Semantics 2026

## Why
The current orchestration engine relies on a natural-language, prompt-driven continuous reasoning loop — the monolithic `RunToolLoop` (`server/internal/orchestrator/llmrunner/toolloop.go:104`). Observed failure modes:

- **Semantic drift**: downstream agents repeatedly re-analyze the repository from natural-language context, so intent degrades hop by hop.
- **Budget starvation**: a single global iteration budget (default 6, raised to 8 for coding steps in `runner.go`) is shared across discovery and implementation, so exploration exhausts it before edits begin.
- **Fragile salvage**: partial successes are recovered via Git commits (`steps/patch_retry_loop.go` salvage path + `orchestrator/checkpoint/`), which couples recovery to Git identity and a clean worktree; salvage fails when either is unavailable.
- **Physical coupling**: planner output names concrete file paths directly, so plans break whenever repository layout differs from planner assumptions.

## What Changes

### Issue 1: Monolithic Tool Loop
- Dismantle the global `RunToolLoop` as the single execution driver.
- Introduce a Runtime State Machine with explicit phases (`DISCOVERY` → `PLAN_READY` → `IMPLEMENTATION` → `VALIDATION`) and **phase-specific** iteration budgets and tool access.
- Migrate call sites incrementally behind a config flag (`execution.state_machine_enabled`); `RunToolLoop` is removed only after all steps are migrated.

### Issue 2: Natural-Language Semantic Drift
- Deprecate free-form prompt/JSON handoff from the Planner.
- Introduce `Execution IR` — a strict, schema-validated semantic execution contract.
- Introduce a `Prompt Compiler` that renders `Execution IR` into provider-specific prompts (single rendering point, no ad-hoc prompt assembly downstream).

### Issue 3: Tight Coupling to the Physical Repository
- Introduce an `Intent Resolver` that translates semantic planner intents ("Create UserRepository") into physical file paths and operations **before** implementation begins. Resolution failures surface at `PLAN_READY`, not mid-edit.

### Issue 4: Fragile Checkpointing
- Replace Git-based recovery with `ExecutionSnapshot`: pure state-based checkpointing (workspace diff + tool history + runtime state) that is replayable without Git.
- Git is demoted to an **export-only** concern at terminal states (PR creation via `gitops/`).

## Non-Goals
- No change to Planner task decomposition or the execution graph shape.
- No change to the sandbox runtime (`internal/sandbox/`) beyond snapshot read/write hooks.
- Git-based PR export (`orchestrator/gitops/`) is retained; only Git-as-recovery is removed.

## Capabilities

### New Capabilities
- Deterministic state transitions with per-phase budgets (Runtime State Machine).
- Provider-agnostic prompt rendering (Prompt Compiler).
- Non-Git execution checkpointing and replay (ExecutionSnapshot).
- Semantic-to-physical target resolution (Intent Resolver).

### Modified Capabilities
- `RunToolLoop` call sites migrate to phase-scoped state machine execution (flag-gated during rollout).
- `orchestrator/checkpoint/` recovery switches its persistence backend from Git commits to `ExecutionSnapshot`.

### Removed Capabilities
- Git-dependent salvage of partial tool-loop results (`patch_retry_loop.go` salvage-checkpoint path).

## Impact

| Area | Files Affected |
|------|----------------|
| Tool loop | `server/internal/orchestrator/llmrunner/toolloop.go`, `server/internal/orchestrator/llmrunner/runner.go` |
| Planner | `server/internal/orchestrator/steps/analyze.go`, `server/internal/orchestrator/steps/plan.go` |
| Prompts | `server/internal/prompts/builder.go`, `server/internal/prompts/assembler.go` |
| Recovery | `server/internal/orchestrator/checkpoint/`, `server/internal/orchestrator/steps/patch_retry_loop.go` |
| Orchestration | `server/internal/orchestrator/llm_step.go` |
| Config | `server/pkg/config/config.go`, `server/pkg/config/config.yaml` |
