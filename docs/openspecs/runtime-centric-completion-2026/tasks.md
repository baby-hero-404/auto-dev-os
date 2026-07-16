# Tasks: Runtime-Centric Completion 2026

> Dependency order: 1.1 (gate) informs 5.1 (retirement — follow-up, blocked on the gate passing,
> out of this change's initial scope). 2.1 (Intent Resolver reversal) is independent of 1.x and
> 3.x. 3.1 (review objectives) and 3.2 (MaxFiles) are independent of each other. 4.1 (semantic
> hash) is independent but benefits from 2.1 landing first (resolved targets become more
> trustworthy as a hash input).

## P0 — Critical

### Task 1.1: Build the State Machine Rollout Gate
>  Status: Completed
> Links to: REQ-001

**Acceptance Criteria:**
- [x] `TaskRepo.ListRecentByStatus(ctx, statuses []string, limit int) ([]models.Task, error)`
      added to `internal/repository/task.go` (no existing method covers "most recent terminal
      tasks").
- [x] `WorkflowRepo.FindLogsByPattern(ctx, taskIDs []string, level, messagePrefix string)
      ([]models.TaskLog, error)` added to `internal/repository/workflow.go`, branching on
      `r.fileRoot` exactly like `CreateLog`/`TailLogs` already do (DB `Where` vs. JSONL scan) —
      **not** raw SQL against `*sql.DB`, since `cfg.Logging.FileRoot` defaults to
      `<DataRoot>/logs` and is active by default in this deployment (`config.go:208`,
      `cmd/api/main.go:90`); a SQL-only implementation would silently return zero rows here.
- [x] `EvaluateStateMachineGate(ctx, tasks *repository.TaskRepo, logs *repository.WorkflowRepo,
      sampleSize int, thresholdPct float64) (GateResult, error)` implemented in
      `server/internal/orchestrator/rollout/gate.go`, using the two methods above.
- [x] `TopViolationTypes` buckets the returned violation rows' `Message` text against known
      phrasings (`tool %s is not permitted`, `out-of-scope write`) client-side.
- [x] Standalone CLI entry point (`cmd/rollout-gate/main.go`, mirroring `cmd/migrate/main.go`'s
      structure) that runs the gate and prints `GateResult` as JSON — no new DB table.
- [x] Unit tests for `EvaluateStateMachineGate` against a fixture `WorkflowRepo`/`TaskRepo`
      (in-memory or `sqlite`/testcontainer, whichever this repo's existing repository tests use)
      covering: pass case, fail case, zero-calls edge case (division-by-zero guard).
- [x] Unit tests for `WorkflowRepo.FindLogsByPattern` in **both** modes (`fileRoot==""` DB path
      and `fileRoot!=""` JSONL path) — mirroring how `workflow_test.go` already tests `TailLogs`
      in both modes (`workflow_test.go:19,80,124` call `SetLogFileRoot`).
- [x] Runbook note (README or `docs/`) describing how to run the gate and interpret the result —
      this is the artifact that replaces "pending release cycle" with a defined procedure.

### Task 2.1: Make Intent Resolver Authoritative
>  Status: Completed
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] `ResolveIntent` in `intent_resolver.go` always runs token-matching against
      `analysis.AffectedFiles`; the `len(targetFiles) > 0` early-return (`intent_resolver.go:97-99`)
      is removed. Signature changes to `(resolved []string, dropped []string, err error)` —
      `ResolveIntent` stays I/O-free per its own documented principle
      (`intent_resolver.go:90-91`); it returns disagreement data, it does not log it.
- [x] LLM-suggested `unit.TargetFiles` are kept only when corroborated by `analysis.AffectedFiles`;
      resolved set = union(token matches, corroborated suggestions); uncorroborated suggestions
      are returned via the new `dropped` return value.
- [x] `ResolveExecutionIRTargets` (`intent_resolver.go:139-161`) aggregates `dropped` per node
      into a new `map[node_id][]string` return value, still performing no I/O.
- [x] `plan.go`'s `PlanStep.Execute` (which already holds `s.log Logger`, `plan.go:28`, and
      already logs this function's errors at `plan.go:235,245`) logs each non-empty `dropped`
      entry via `s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", ...)` — this is where
      corroboration-disagreement observability actually lands, not inside the resolver.
- [x] `IntentResolutionError.Reason` strings reworded — they currently narrate the old fallback
      order ("attempted unit target_files (none found)…", `intent_resolver.go:106,121`).
- [x] `analyze.go:322-332`'s hard validation failure on empty `target_files` is removed;
      `execution_ir.schema.json` marks `target_files` optional.
- [x] `analyze_step_test.go` updated: remove the "empty target_files fails" assertion, add
      "empty target_files with populated Capability/Objective passes" assertion.
- [x] New tests in `intent_resolver_test.go`: `TestResolveIntent_DropsUncorroboratedLLMSuggestion`
      and `TestResolveIntent_KeepsCorroboratedLLMSuggestion`, asserting on the new `dropped`
      return value directly (not on a log call).
- [x] New test (`plan_test.go` or equivalent): non-empty `dropped` from
      `ResolveExecutionIRTargets` produces the expected `s.log.Log(..., "warn", ...)` call.
- [x] Existing `TestResolveIntent_Resolvable/Ambiguous/Unresolvable` still pass, updated for the
      new three-return-value signature.
- [x] `go test ./internal/orchestrator/steps/... ./pkg/models/...` green.

## P1 — High

### Task 3.1: Inject Execution Unit Objectives into Review
>  Status: Completed
> Links to: REQ-003

**Acceptance Criteria:**
- [x] **No model change**: `review.go` renders objectives from `frozen.ExecutionUnits` — the
      `FrozenContext` already snapshots the full unit list (`task.go:235`), each unit carrying
      `Objective`.
- [x] `review.go`'s instruction builder appends one "UNIT OBJECTIVES" block listing every frozen
      unit with a non-empty `Objective`, keyed by unit ID, in the same guarded style as the
      existing AC/boundary blocks.
- [x] Legacy-task path (no `FrozenContext`, or no unit with a non-empty `Objective`) produces
      byte-identical instructions to today — verified by an existing/extended golden test in
      `review_test.go`.
- [x] New test asserting the objectives block lists all units with objectives (multi-unit case,
      matching the merged-diff reality of `code_backend_0..n`).

### Task 3.2: Enforce MaxFiles at PLAN_READY
>  Status: Completed
> Links to: REQ-004

**Acceptance Criteria:**
- [x] After `ResolveIntent` succeeds inside `ResolveExecutionIRTargets`'s per-IR loop
      (`intent_resolver.go:139-161`), compare `len(targets)` against `unit.Constraints.MaxFiles`;
      `MaxFiles <= 0` means unenforced (matches today's default behavior for units that don't
      set it).
- [x] Over-budget resolution joins the same aggregated error as unresolvable-intent (hard
      failure flag-on, warn log flag-off — same semantics as `intent_resolver.go:133-138`); the
      error names both counts and the node ID.
- [x] Unit tests: exactly-at-budget passes, one-over-budget fails, `MaxFiles=0`/unset always
      passes regardless of resolved count.

## P2 — Medium

### Task 4.1: Add SemanticHash Alongside PromptHash
>  Status: Completed
> Links to: REQ-005

**Acceptance Criteria:**
- [x] `ExecutionSnapshot.SemanticHash` field added (`pkg/models/ir.go:128-136`) — **not**
      `ExecutionIR` (`ir.go:52-59`), which has no hash field today; `PromptHash` already lives on
      `ExecutionSnapshot`, so `SemanticHash` joins it there.
- [x] **No DB migration**: `ExecutionSnapshot` has no `gorm` tags and is not a SQL table — it is
      JSON-marshaled via `r.SaveArtifact(...)` (`statemachineloop.go:470`). An initial pass added
      `migration/000013_add_semantic_hash.{up,down}.sql` plus `SemanticHash` columns on
      `WorkflowCheckpoint`/`WorkflowArtifact` (`pkg/models/workflow.go`) — review found neither
      column was ever written (`checkpoint.Store.SaveArtifact` never sets them) nor read (resume
      logic in `llm_step.go` reads `snap.SemanticHash` from the decoded JSON payload, not the DB
      column). Removed the migration and both fields as dead code; confirmed via
      `go build ./...`/`go test ./...` and `artifact_test.go` reverted to its pre-column mocks.
- [x] `ComputeSemanticHash(ir models.ExecutionIR, resolvedTargets []string) string` implemented
      per design.md — sorts `resolvedTargets`, `ir.Acceptance`, and `ir.Constraints` (all
      `[]string` per the real model, `ir.go:56-57`) before hashing, since schema validation
      doesn't guarantee input order.
- [x] Computed and stored on `ExecutionSnapshot` at the same call site that already computes
      `promptHash` (`statemachineloop.go:456-468`, before `snapshot := models.ExecutionSnapshot{...}`
      is built) and at the resume call site (`llm_step.go:107`).
- [x] Resume logic: `PromptHash` mismatch + `SemanticHash` match → log "semantically unchanged,
      skipping re-reasoning" and resume as if `PromptHash` matched; `SemanticHash` mismatch →
      unchanged full-replay behavior.
- [x] Unit test: two IRs with different Prompt Compiler wording but identical
      Node/Intent/Acceptance/Targets/Constraints produce equal `SemanticHash`, differing
      `PromptHash`.
- [x] Unit test: changing any one of `{NodeID, Intent, Acceptance, resolved targets, Constraints}`
      changes `SemanticHash`.

## Follow-up (tracked here, not executed in this change)

### Task 5.1: Retire RunToolLoop and Legacy Prompt Assembler
> ❌ Status: Blocked on Task 1.1's gate passing for one full release cycle at 100% rollout
> Links to: REQ-002

**Acceptance Criteria (for when unblocked):**
- [ ] `EvaluateStateMachineGate` has reported "pass" continuously for one release cycle with
      `state_machine_enabled=true` on all traffic.
- [ ] `llmrunner/toolloop.go` (`RunToolLoop`) and its exclusive callers removed.
- [ ] `internal/prompts/builder.go`/`assembler.go` removed or reduced to whatever
      `PromptCompiler` still delegates to; golden tests updated accordingly.
- [ ] No remaining call site can reach the shadow-FSM code path (`updateShadowSM`,
      `runner.go`'s `runAgentic`) — `runStateMachine` is the only execution path left.
