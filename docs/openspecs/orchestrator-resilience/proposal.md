# Proposal: Orchestrator Resilience

> **v1.1 (Code-Verified)** — This revision corrects two factual errors found in v1.0 by
> re-reading the actual source (see `docs/reports/deep_investigation_report.md` follow-up
> verification): the Issue 4 failure mechanism was misattributed, and Issue 5 incorrectly
> claimed the Review step lacked `FrozenContext` when it already has it.

## Why
Recent deep investigation of the prompt assembly pipeline and orchestrator workflow revealed several edge cases that can lead to crashes, context loss, and incorrect task routing. These issues undermine the reliability of the agentic loops, especially when token budgets are tight or when LLMs produce unexpected markdown formats.

## What Changes

### Issue 1: Subtask Routing Fragility
- **Root cause (corrected):** `extractSpecsSectionForSubtask`'s local `isRole` closure (`server/internal/prompts/helpers.go:193-206`) is an independent, drifted reimplementation of the canonical heading classifier `classifyHeading` (`server/internal/workflow/parser.go:10-23`), which is what actually decides `subtasks["backend"]`/`["frontend"]` bucketing via `ParseTasksMD`. The two keyword lists have diverged (e.g. `isRole` is missing "component", "page", "view", "style", "css", "layout", "migration", "model", "service", "handler" and the no-diacritic Vietnamese variants that `classifyHeading` already recognizes), so a heading correctly bucketed as frontend by `classifyHeading` can be miscounted as backend by `isRole`, desynchronizing the index used to slice `SpecsMD` from the index used to slice `TasksMD`.
- **Fix:** Export `classifyHeading` as `workflow.ClassifyHeading` and have `extractSpecsSectionForSubtask` call it directly instead of maintaining a second, incomplete keyword list. This guarantees the two role-bucketing paths can never drift apart again.

### Issue 2: Execution Manifest Dropped on Budget Exhaustion
- Mark the `Execution Manifest` prompt section as immutable (`IsImmutable: true`) in `builder.go` so that the `optimizeBudget` function never drops critical execution contracts.
- **Note:** Execution Manifest is `Priority: 40`, one of the lowest-priority (latest-dropped) mutable sections — several higher-priority sections (Semantic Context, Repository Structure, Retrieved Memories, Tasks Progress, Clarifications, JIT Skills, Step Prompt) are dropped first. This fix closes the remaining exposure rather than fixing a frequently-hit bug.

### Issue 3: Type Assertion Crash in Plan Output Parsing
- Add safe type assertions and error handling in `code_backend.go` and `code_frontend.go` when parsing subtask arrays from the plan output.
- **Note:** `orchestrator/worker.go` has a top-level `recover()`, so today's unchecked assertion fails the task (logged, `TaskStatusFailed`) rather than crashing the process. The fix still matters: it turns an opaque panic/stack trace into an actionable warning and lets the step fail cleanly.

### Issue 4: Sandbox Commit Failure Data Loss
- **Mechanism (corrected):** The data-loss path is *not* the intra-run attempt-retry loop in `worker.go` — that loop re-runs the workflow engine without touching git state, and `SetupRoleWorktrees` explicitly preserves an already-valid worktree (`repoutil/worktrees.go:57-63`). The actual loss happens when a job is **resumed from checkpoints on a fresh run** (`worker.go:275-358`): if `commitSandbox` fails after `runPatchRetryLoop` succeeds, the coding step is marked `Failed`, so no `commit_hash` checkpoint is recorded for it (`worker.go:177-192` only checkpoints on `Success`). On resume, `worker.go:329-357` finds the *previous* successful commit checkpoint and calls `RestoreGitCheckpoint`, which does `git checkout / reset --hard / clean -fd` (`repoutil/worktrees.go:189-192`) — permanently discarding the uncommitted patch from the failed step.
- **Fix:** `RestoreGitCheckpoint` now snapshots a dirty worktree to a throwaway `rescue/*` branch before the destructive reset, so uncommitted work is preserved in git history (recoverable) even though the active worktree still resets cleanly for retry. `CommitRoleWorktrees` also gets a small retry-with-backoff around the commit itself, since the underlying failure is usually transient (lock/IO contention).

### Issue 5: Stale Context in the Fix Step (corrected scope)
- **Correction:** The Review step (`review.go:229-243`) already loads `FrozenContext` and injects `AcceptanceCriteria`/`ExecutionBoundaries` — this was already fixed on `master` and does not need further work. The **Fix** step is the one that's missing it: `fix.go` never calls `LoadFrozenContext`, and separately, `builder.go`'s generic Execution Manifest branch excludes `acceptance_criteria`/`execution_boundaries` for *any* `isCodingStep` (`builder.go:638-645`), a bucket `StepFix` shares with `code_backend`/`code_frontend` even though (unlike those two) Fix has no subtask index and so never receives the per-subtask `extractSpecsSectionForSubtask` context either.
- **Fix:** Mirror `review.go`'s `LoadFrozenContext` + Acceptance Criteria/Execution Boundaries injection into `fix.go`'s instruction, and special-case `StepFix` in `builder.go`'s coding-step manifest branch so it also gets `acceptance_criteria`/`execution_boundaries`.

## Capabilities

### Modified Capabilities
- Context Optimization: Now guarantees retention of Execution Manifest.
- Subtask Routing: Delegates to the single canonical heading classifier; no more drift between spec-slicing and task-slicing.
- Sandbox State Management: Uncommitted work is preserved (via rescue branch) instead of being silently discarded on checkpoint restore.
- Fix Step Context: Fix now receives Acceptance Criteria and Execution Boundaries, matching what Review already receives.

## Impact

| Area | Files Affected |
|------|----------------|
| Prompts | `server/internal/prompts/helpers.go`, `server/internal/prompts/builder.go` |
| Workflow | `server/internal/workflow/parser.go` |
| Orchestrator | `server/internal/orchestrator/steps/code_backend.go`, `server/internal/orchestrator/steps/code_frontend.go`, `server/internal/orchestrator/steps/fix.go` |
| Repoutil | `server/internal/orchestrator/repoutil/worktrees.go` |
