# Design: Execution Boundary & Target Resolution Hardening

## 1. Architecture Overview
All five fixes strengthen one contract: **the analyze step's output must be executable as written**. Boundary coverage and per-unit targets are enforced at the source (analyze validation), consumed downstream (prompt assembly, intent resolver), and backstopped at runtime (tool-loop stall detection, structural-failure retry policy). No new services or storage; every change is inside the existing orchestrator pipeline.

```
analyze (validate: coverage + per-unit targets)   ← Issues 1, 2 (source of truth)
   └─> plan / ResolveExecutionIRTargets           ← Issue 3 (targets-first resolution)
         └─> BuildInitialMessages (per-unit files) ← Issue 2 (consumption)
               └─> RunToolLoop / runStateMachine   ← Issue 4 (stall backstop)
                     └─> worker retry loop         ← Issue 5 (no blind retries)
```

## 2. Issue 1 — Boundary Coverage Validation

### 2.1 Placement
`AnalyzeStep` already routes LLM responses through `llmrunner.RunToolLoop` with an analyze-specific `Validate` hook (`steps/analyze.go`, wired at `analyze.go:202`). Coverage becomes one more business-validation rule in that hook, next to the existing presence-only check (`analyze.go:280-283`) — reusing the loop's corrective-feedback path instead of inventing a new one.

### 2.2 Rule
```go
// validateBoundaryCoverage returns an error naming every affected_files[].file
// (and, with Issue 2, every execution_units[].target_files[] entry) not covered
// by any execution_boundaries[].root.
func validateBoundaryCoverage(analysis models.TaskAnalysis) error
```
- Coverage test: `path.Clean` both sides; file is covered when its path has a boundary root as a directory prefix (`strings.HasPrefix(file, root)` with a trailing-`/` guard so `internal/` does not cover `internals/`). Empty `root` covers everything (explicit whole-repo boundary).
- Error message template (also the re-prompt feedback):
  `"execution_boundaries do not cover: cmd/zentao-sync/main.go (declared roots: internal/). Widen an existing boundary or add one so every affected/target file is covered."`
- On budget exhaustion with coverage still broken, `AnalyzeStep` returns a hard error listing the uncovered files (REQ-001 scenario 2). This is deliberate fail-fast: the e69924ba incident shows a contradictory contract costs 30+ downstream LLM calls; failing analyze costs zero.

## 3. Issue 2 — Per-Unit Target Files

### 3.1 Schema
`models.ExecutionUnit` (`server/pkg/models/task.go`) gains:
```go
TargetFiles []string `json:"target_files"` // files this unit creates or modifies; required, non-empty
```
The analyze prompt's output-schema section documents the field; the analyze `Validate` hook rejects any unit with a missing/empty list (REQ-002), and each entry also passes `validateBoundaryCoverage`.

### 3.2 Prompt scoping
`Runner.BuildInitialMessages` (`llmrunner/runner.go:72`) currently iterates `analysis.AffectedFiles` for every coding step. Change: for steps with a subtask index (`code_backend_N` / `code_frontend_N`), resolve the unit and use its `TargetFiles`:

```go
// unitForStep maps code_backend_2 -> the 3rd execution unit whose
// execution_profile.agent == "backend", reusing the same index convention
// as prompts.extractSubtaskIndex (helpers.go:167) and plan.go's unit->step mapping.
func unitForStep(analysis models.TaskAnalysis, stepID string) (*models.ExecutionUnit, bool)
```
- Fallback: when the unit has no `TargetFiles` (legacy analyses persisted before this change), fall back to today's task-wide `AffectedFiles` behavior — old queued tasks keep working.
- The existing "[NEW FILE — does not exist yet]" annotation logic is unchanged; only the file list feeding it narrows.

### 3.3 Non-goal
The `fix` step keeps the task-wide view (its scope is "whatever review flagged", which may span units). Only `code_backend_N`/`code_frontend_N` prompts narrow.

## 4. Issue 3 — Intent Resolution

### 4.1 Resolution order (per ExecutionIR node)
In `ResolveIntent` (`steps/intent_resolver.go:80`):
1. **Unit targets:** if the unit owning this node declares `TargetFiles`, return them (they are already boundary-validated by Issue 1+2). Resolution succeeds without touching the capability string.
2. **Identifier tokens:** existing `intentTokens` + `pathMatchesTokens` behavior, kept for identifier-style capabilities and legacy analyses.
3. **Structured failure:** `IntentResolutionError.Reason` now records both attempts, e.g. `"no target_files declared; token matching failed: no workspace file matched tokens [...]"`.

### 4.2 Natural-language detection
`intentTokens` short-circuits when the capability is prose rather than an identifier — heuristic: contains ≥3 whitespace-separated words after separator normalization, or any non-ASCII letter. For such strings step 2 is skipped entirely (its syllable tokens are noise, per the incident), making step 1 or step 3 the only outcomes. The heuristic is deliberately cheap; correctness does not depend on it because step 1 dominates whenever Issue 2's schema is in effect.

## 5. Issue 4 — Tool-Loop Stall Safeguard

### 5.1 Shared helper
One new file `llmrunner/stallguard.go`:
```go
// stallGuard tracks per-run call outcomes to intercept successful-but-non-progressing
// tool calls before they consume iteration budget.
type stallGuard struct {
    succeeded map[string]int // "name:sha256(args)" -> turn of first success
}

// Check returns (interceptResult, true) when the call must not be executed:
//   - search_replace with search == replace (no-op edit)
//   - exact repeat of an already-successful read-only call (list_files, read_file, run_lint...)
// The interceptResult is the corrective tool-result text to feed back.
func (g *stallGuard) Check(name, argsJSON string) (string, bool)

// RecordSuccess registers a successful non-"Error:" call.
func (g *stallGuard) RecordSuccess(name, argsJSON string, turn int)
```
Both `RunToolLoop` (`toolloop.go`, next to the existing `failureCounts`/`readMemo` checks) and `runStateMachine` (`statemachineloop.go`, same position in its per-call sequence) call `Check` before `ExecuteTool` and `RecordSuccess` after. This is intentionally the first concrete step of the planned tool-loop dedup (PLAN-tech-debt-sse-fixes Phase 3B) — new shared behavior lands once, in one place.

- `read_file` repeats remain handled by the existing `readMemo` (range-aware); `stallGuard` covers the other read-only tools and exact-duplicate calls generally.
- Corrective texts:
  - No-op edit: `"Error: this search_replace is a no-op (search and replace are identical). The file already contains this content. Make a real change, or move on."`
  - Repeat call: `"You already ran %s with these exact arguments at turn %d and got a successful result. Re-running it returns no new information. Either write to a file within your execution boundary now, or explain in your final summary why you cannot."`
- Intercepted calls still consume the iteration (same rationale as the existing circuit-breaker NOTE at `toolloop.go:178-181`).

### 5.2 EditsApplied accuracy (REQ-M01)
The no-op check runs *before* execution, so a no-op `search_replace` can no longer reach the `editsApplied` append in either loop. The salvage paths (`toolloop.go:220-224`, `statemachineloop.go:341-354`) need no change beyond that — `Partial` is computed from `editsApplied`, which now only contains real edits.

## 6. Issue 5 — Structural-Failure Retry Policy

### 6.1 Classification
New sentinel in the workflow package (next to `ErrPaused`/`ErrReviewFixLoop`):
```go
// ErrNoProgress marks a step failure where the agent applied zero workspace
// edits — a structural blocker (contradictory boundaries, missing targets),
// not a transient one. Retrying with an unchanged prompt cannot succeed.
var ErrNoProgress = errors.New("step failed with no workspace progress")
```
The coding/fix step failure path (patch retry plumbing in `steps/patch_retry_loop.go`) wraps its terminal error with `ErrNoProgress` when the loop result shows zero applied edits (`ToolLoopResult.EditsApplied` empty / no salvage checkpoint created).

### 6.2 Worker behavior
In the worker retry loop (`worker.go:430-435`), before scheduling a retry:
```go
if attempt >= 1 && errors.Is(err, workflow.ErrNoProgress) {
    o.log(ctx, task.ID, &job.ID, "warn", "structural failure (no workspace progress); skipping remaining retries")
    break
}
```
One re-attempt is still allowed (attempt 0 → 1) because the retry now differs from the original: per §6.3 it carries the prior error. Attempts beyond that are provably identical inputs → skipped.

### 6.3 Error carry-forward
The worker records the failed attempt's terminal error; on retry it is passed into the engine run context and appended by `BuildInitialMessages` to the step instruction:
```
PREVIOUS ATTEMPT FAILED with the following error — address it explicitly before anything else:
Error: execution boundary violation on "cmd/zentao-sync/main.go": ...
```
Transport: a `map[string]any` key on the engine's run input (`worker.go:406-408` already passes `task_id`/`agent_id`/`job_id` this way), read by the step and threaded to the runner as part of `instruction`. No schema/persistence change — the error only needs to survive one in-process retry.

## 7. Tradeoffs & Decisions
- **Repair-then-fail vs auto-widen boundaries (Issue 1):** auto-widening the boundary to cover uncovered files would always "succeed" but silently erodes the security property boundaries exist for (`policy_engine.go` write-scope enforcement). Letting the LLM repair its own output keeps a human-auditable contract; hard failure after budget is the honest terminal state.
- **Schema field vs smarter resolver (Issue 2/3):** deriving per-unit targets purely in the resolver (better tokenization, embeddings, etc.) was rejected — the planner LLM already knows which files each unit owns at generation time; asking it to state that explicitly (`target_files`) is strictly cheaper and language-independent. The tokenizer fix is kept only as a fallback for legacy/identifier-style analyses.
- **One re-attempt for `ErrNoProgress` instead of zero (Issue 5):** with §6.3 the first retry genuinely differs (it sees the blocker), so it retains a real chance of success — e.g. the model can explain in its summary why a file outside the boundary must change, which the policy error text explicitly invites. Identical-input retries beyond that are pure waste and are cut.
- **Stall guard consumes iterations (Issue 4):** matching the existing circuit-breaker decision (`toolloop.go:178-181`) — a model repeating blocked/no-op calls must still hit the cap, otherwise it can loop forever.
