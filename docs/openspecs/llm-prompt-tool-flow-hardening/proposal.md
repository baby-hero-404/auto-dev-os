# Proposal: LLM Prompt & Tool Flow Hardening

## Why

`docs/reports/llm_prompt_tool_flow_audit.md` (2026-07-12) traced the full LLM-call, prompt-assembly, and tool-calling flow against the current source tree (not logs) and confirmed that the previously-flagged "coding steps are single-shot, only `analyze` gets tool calls" architecture gap is fixed — but found **three new correctness/safety bugs introduced or exposed by that same fix**, plus a set of secondary reliability and maintainability gaps. Two of the three critical findings were independently re-verified with direct `grep` against source (not just agent-reported) before this proposal was written:

- `internal/gateway/gateway.go` has zero references to `ExcludeModelID` — the Review step's self-review-bias mitigation (Harness Independence) is fully wired end-to-end but the production gateway never reads it.
- `internal/orchestrator/llmrunner/toolloop.go:98-99` decrements the iteration counter (`i--`) whenever every tool call in a round was blocked by the circuit breaker — meaning a model stuck repeating an already-blocked call never hits `maxIterations`.

The remaining findings (silent error swallowing in prompt assembly, in-memory-only credential cooldowns, disagreeing transient-error classifiers, no execution-time tool capability check, no partial-result salvage, unbounded tool-loop token growth, a 424-line single-return `collect()`, scattered magic numbers, a still-duplicated `AnalyzeStep` loop) are documented with file:line evidence in the same report.

## What Changes

### Issue 1: Harness Independence Not Enforced in Production
- Port the `ExcludeModelID` filtering logic from `pkg/llm/router.go` (dead-code `Gateway`, correct implementation) into `internal/gateway/gateway.go`'s route-entry selection (the real, production `AIGateway`).
- Preserve the graceful single-model-in-level-group fallback behavior already proven correct in `pkg/llm/router_test.go`.
- **Enhancement:** when the graceful fallback fires (no other model available, so the excluded/coder model is reused for review), record this as a `self_review_fallback: true` flag — not just a `slog` warning — propagated into the Review step's output and surfaced the same way `review_limit_exceeded` already is (PR body warning, checkpoint state). A log line disappears into the noise; a persisted flag lets a human reviewer or a dashboard actually filter for tasks whose review integrity was compromised.

### Issue 2: Tool-Loop Circuit Breaker Unbounded-Cost Loophole
- Fix `toolloop.go` so that a round where every tool call was blocked by the circuit breaker still counts toward `maxIterations`, instead of being "un-consumed" via `i--`.
- Extend the circuit breaker to also throttle path-less tools (`run_tests`, `run_build`, `run_lint`) that currently bypass it entirely.

### Issue 3: Prompt Assembly Silently Swallows Errors
- `PromptAssembler.collect()` (`builder.go:401-824`) must surface (at minimum: log at `error`/`warn`) the failure of `loadRules()`, `resolveSkills()`, `RetrieveContext()`, `GetRepoMap()`, and `json.Unmarshal(task.Analysis, ...)` instead of silently proceeding with a degraded prompt.
- Priority: `loadRules()` failure is the most severe (a task can be sent to the LLM with zero governance/security rules and no signal this happened).

### Issue 4: Credential Cooldown Persistence & Transient-Error Classification
- Persist per-(credential, model) cooldowns instead of the current in-memory-only `CredentialPoolService.modelCooldowns` map (lost on restart, not shared across replicas).
- Unify the two disagreeing `isTransientError` implementations (`internal/gateway/gateway.go:353-365` narrow / `internal/orchestrator/llmrunner/runner.go:402-419` broad) into one canonical classifier used at the gateway (credential-aware) layer.
- **Enhancement:** the in-process read cache in front of the persisted store must have an explicit, named TTL (not "cache indefinitely until a local write invalidates it") so that a cooldown set by one replica becomes visible to other replicas within a bounded, documented window instead of only on that replica's own next write.

### Issue 5: Tool Execution Authorization Gap
- Add an execution-time role/capability check inside `tool.Registry.Execute` (or a wrapping executor) for every tool call, not just `search_replace`/`create_file` (currently the only two covered by `boundary_tool_executor.go`).

### Issue 6: Tool-Loop Failure Handling
- On `RunToolLoop` iteration exhaustion, detect whether any edit tool calls already succeeded and surface a partial result instead of always hard-failing the step, discarding completed work.
- When `patch_retry_loop.go` retries after a post-hoc test failure, carry forward a compact note of files already read in the prior attempt so the model doesn't re-discover them from scratch.
- **Enhancement:** before running targeted tests against a salvaged partial result, create a secondary git checkpoint of the worktree state first. Targeted tests can run arbitrary build/test commands that may hang or leave the sandbox dirty — without a checkpoint taken *at the moment of salvage*, a corrupted test run has nothing safe to revert to and either loses the partial edits or leaves the worktree in an undefined state.

### Issue 7: Tool-Loop Token/Cost Growth Controls
- Cap tool-result size before appending to the loop's message history, especially `run_tests`/`run_build` stdout+stderr (currently unbounded).
- Add read-memoization within a single tool-loop run so repeated `read_file` calls on the same path don't re-consume tokens.

### Issue 8: Prompt Builder Maintainability
- Decompose `PromptAssembler.collect()` (424 lines, single `return`) into named per-concern helpers (base/role prompts, layered rules, reviewer-vs-general context routing, semantic context, repo map), each returning `(PromptSection, error)`.
- Promote scattered magic numbers (priority/render-order literals, JIT skill limit, snippet caps, repo-map token clamps, dedup overlap threshold) to named constants, matching the pattern already used for `defaultPromptBudget`/`promptBudgetReserveRatio`.

### Issue 9: Duplicate Agentic Loop in AnalyzeStep
- Migrate `AnalyzeStep.runAnalyzeLLMLoop` (`analyze.go:242-440`) onto the shared `RunToolLoop`, removing the second hand-rolled implementation of the same pattern.

### Issue 10: Minor Reliability & Observability Cleanups
- Add an explicit `http.Client.Timeout` to `NineRouter` (currently the only provider without one).
- Switch outer retry backoff sleeps (`llmrunner/runner.go:125,228`) to ctx-aware waiting, matching the gateway-level backoff.
- Record one usage/cost row per attempted provider/credential (not just the last) for real fallback-chain observability.
- Replace stray `fmt.Printf` debug/warning output (`gemini.go:169`, `router.go:191`) with structured `slog` logging.

## Capabilities

### New Capabilities
- Execution-time tool capability enforcement covering all tools (not just edit tools).
- Partial-result salvage path when the tool loop exhausts its iteration budget after making real edits.
- Persistent (restart-surviving, replica-shared) credential cooldown state.

### Modified Capabilities
- `AIGateway` model/route selection: now honors `ExcludeModelID` (Harness Independence).
- Tool-loop circuit breaker: fully-blocked rounds now count toward the iteration budget; path-less tools are now throttled too.
- `PromptAssembler.collect()`: surfaces internal failures instead of silently degrading; decomposed into testable helpers.
- Transient-error classification: single canonical implementation instead of two disagreeing ones.
- `AnalyzeStep`: uses the shared `RunToolLoop` instead of its own loop implementation.

### Removed Capabilities
- None — this is a hardening/refactor pass, not a capability removal.

## Impact

| Area | Files Affected |
|------|----------------|
| Gateway / Routing | `server/internal/gateway/gateway.go`, `server/internal/gateway/cooldown_worker.go` |
| Credential Pool | `server/internal/service/credential_pool.go`, `server/internal/service/credential_router.go`, `server/internal/repository/provider_credential.go` |
| Tool Loop | `server/internal/orchestrator/llmrunner/toolloop.go`, `server/internal/orchestrator/llmrunner/runner.go` |
| Tool Registry | `server/internal/tool/registry.go`, `server/internal/tool/capability.go`, `server/internal/orchestrator/steps/tool_executor.go`, `server/internal/orchestrator/steps/boundary_tool_executor.go` |
| Prompt Assembly | `server/internal/prompts/builder.go`, `server/internal/prompts/assembler.go`, `server/internal/prompts/helpers.go` |
| Retry Loop | `server/internal/orchestrator/steps/patch_retry_loop.go` |
| Analyze Step | `server/internal/orchestrator/steps/analyze.go` |
| Providers | `server/pkg/llm/nine_router.go`, `server/pkg/llm/gemini.go`, `server/pkg/llm/router.go` |
