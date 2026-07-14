# Proposal: Execution Boundary & Target Resolution Hardening

## Why
Task `e69924ba-3dae-496c-8684-b9f294b27ef7` ("zentao auto tool", Go + SQLite sync service) failed after 33 implementation LLM calls having produced only a 3-line `go.mod`. Full forensics in `docs/reports/task-e69924ba-execution-boundary-failure-report.md`; the failure decomposes into five defects, none of which are model-quality issues:

1. **Self-contradicting analyze output.** The `analyze` step emitted a single execution boundary (`root: "internal/"`) while its own `affected_files` required creating `cmd/zentao-sync/main.go` — outside every boundary. Every attempt to create the entrypoint was rejected by the policy engine (`patch/policy_engine.go:204`: *"file is outside of all approved execution boundaries"*). Nothing validates boundary coverage today: `analyze.go:280-283` only checks that the `execution_boundaries` key exists.
2. **No per-unit file targets.** All three `code_backend_N` prompts received the same task-level `affected_files` list (3 files, all belonging to unit `init-core`). Units `api-clients` and `sync-engine-scheduler` — most of the scope — had no file target matching their objective and never created a single relevant file.
3. **Intent resolver cannot handle natural-language capabilities.** `intentTokens()` (`steps/intent_resolver.go:23-55`) splits identifier-style names; fed the Vietnamese sentence *"Thiết lập cấu trúc dự án và SQLite"* it produced syllable tokens (`[thiết, lập, cấu, trúc, …]`) that can never substring-match an English file path. All 3 nodes failed resolution (log line: *"intent resolution incomplete"*). Today this is warn-only, but hard enforcement is planned with the state machine rollout — this task would then fail at `PLAN_READY`.
4. **Tool loop has no stuck-detection for *successful* no-progress calls.** `code_backend_0` spent 4 of 8 iterations on byte-identical no-op `search_replace` calls (search == replace); all 21 `fix` calls across 3 attempts were repeated `list_files`. `RunToolLoop`'s safeguards (`failureCounts`, `readMemo`) only trigger on failed calls and duplicate `read_file` — successful thrashing burns the whole budget unimpeded, and no-op edits are even counted as salvageable "edits applied".
5. **Zero-edit `fix` failures are retried unconditionally.** The worker retried `fix` 3× (`worker.go:432`) with an unchanged prompt against the same structural blocker; attempts 2–3 (14 LLM calls, ~50s) had zero chance of success.

## What Changes

### Issue 1: Boundary coverage validation in `analyze`
- Add a coverage check to the analyze step's validation loop: every `affected_files[].file` must fall under at least one `execution_boundaries[].root` (path-prefix match after normalization).
- On violation, feed a corrective validation error back through the existing `RunToolLoop` `Validate` hook so the LLM repairs its own output (widen a boundary or add one) instead of shipping a contradiction downstream.
- If the iteration budget exhausts with coverage still broken, fail the `analyze` step with an explicit error naming the uncovered files — never let coding steps start against a known-contradictory contract.

### Issue 2: Per-unit file targets
- Extend the analyze output schema: each `execution_units[]` entry gains a required `target_files` list (files this unit creates/modifies), validated non-empty and boundary-covered.
- Scope the "Workspace Affected Files" prompt section per coding step: `code_backend_N` receives only its own unit's `target_files` (resolved via the existing `extractSubtaskIndex` step→unit mapping), not the task-wide list.
- `ResolveExecutionIRTargets` consumes `unit.target_files` as first-priority candidates, making resolution independent of capability-string tokenization.

### Issue 3: Intent resolver natural-language fallback
- Detect non-identifier capability strings (multi-word natural language) in `intentTokens` and skip syllable tokenization for them.
- Resolution order per node: (1) the unit's own `target_files` (Issue 2), (2) identifier-token matching (existing behavior, for identifier-style capabilities), (3) structured failure as today.
- Failure messages must state which strategy failed and why, so a future hard-enforcement gate produces actionable errors.

### Issue 4: Tool-loop no-progress safeguard
- Reject no-op `search_replace` calls (search == replace) before execution with corrective feedback; never count them in `EditsApplied`.
- Track successful call signatures (tool name + arguments hash) per run; on an exact repeat of an already-successful read-only call, return a corrective nudge instead of re-executing ("you already ran this with no new information — write to a file within your boundary or explain why you cannot").
- Apply identically in both loops (`RunToolLoop` and `runStateMachine`) — implement once in a shared helper, not twice.

### Issue 5: Zero-edit failure retry policy
- Classify a coding/fix step failure that applied **zero** edits as structural (new sentinel error, e.g. `workflow.ErrNoProgress`) rather than transient.
- The worker retry loop stops retrying on structural failures after the first attempt instead of burning `maxRetries`.
- When a retry does happen, inject the previous attempt's terminal error (e.g. the boundary-violation message) into the next attempt's instruction so the model can react to the actual blocker.

## Capabilities

### New Capabilities
- Analyze-time boundary/coverage contract: `affected_files` ⊆ `execution_boundaries` roots, enforced via the validation loop.
- Per-execution-unit `target_files` schema field, threaded into per-step prompts.
- Tool-loop stall detection for successful-but-non-progressing calls (no-op edits, repeated identical read-only calls).
- Structural-failure classification (`ErrNoProgress`) with early retry termination and error-carry-forward into retry prompts.

### Modified Capabilities
- `AnalyzeStep` validation (`steps/analyze.go`) — coverage check added to schema/business validation.
- `PromptAssembler`/`Runner.BuildInitialMessages` — "Workspace Affected Files" scoped per unit for coding steps.
- `ResolveIntent`/`intentTokens` (`steps/intent_resolver.go`) — target_files-first resolution, natural-language detection.
- `RunToolLoop` (`llmrunner/toolloop.go`) and `runStateMachine` (`llmrunner/statemachineloop.go`) — shared no-progress safeguard; `EditsApplied` excludes no-ops.
- Worker retry loop (`orchestrator/worker.go:430-435`) — structural-failure short-circuit.

### Removed Capabilities
- Unconditional 3× retry of coding/fix steps that made no workspace progress.
- Counting no-op edits toward "salvageable partial result" status.

## Impact

| Area | Files Affected |
|------|----------------|
| Analyze step | `server/internal/orchestrator/steps/analyze.go` |
| Intent resolver | `server/internal/orchestrator/steps/intent_resolver.go` |
| Plan step | `server/internal/orchestrator/steps/plan.go` |
| Models/schema | `server/pkg/models/task.go` (ExecutionUnit.TargetFiles) |
| Prompt assembly | `server/internal/orchestrator/llmrunner/runner.go`, `server/internal/prompts/builder.go` |
| Tool loops | `server/internal/orchestrator/llmrunner/toolloop.go`, `server/internal/orchestrator/llmrunner/statemachineloop.go` |
| Worker retry | `server/internal/orchestrator/worker.go`, `server/internal/workflow/*.go` (sentinel error) |
