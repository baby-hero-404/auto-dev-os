# Design: Analyze-Step Runtime Determinism & Trace Clarity

## Guiding principle
Every change here is grounded in an observed artifact from run `dde2df6c`. Where the v7.0 report proposed new subsystems (Prompt Compiler, Semantic Adapter, Evidence/Decision Ledgers), this design deliberately does **not** — those primitives either already exist (contract, IR, DAG, validator, state machine) or address phases that never ran in the evidence. The work is three surgical hardening changes.

## Issue 1 — Trace iteration vs retry (`llm_trace.go` + call sites)

### Current state (verified)
```go
// llm_trace.go
RetryAttempt int `json:"retry_attempt"`   // :80
RetryAttempt: retryAttempt,                // :113
```
Callers pass semantically different integers into the one `retryAttempt` param:
- `runner.go:337` → `iteration` (tool-loop counter; the analyze path → produced `retry_attempt: 1..9`)
- `statemachineloop.go:177` → `sm.used[currentState]+1` (phase iteration)
- `runner.go:265` → `finalAttempt` (a real retry count)

### Change
Make the trace carry **both** concepts explicitly. Extend the trace struct:
```go
Iteration    int `json:"iteration"`               // per-call loop index, always accurate
RetryAttempt int `json:"retry_attempt"`           // real failure-retry count; 0/1 when not a retry
CallKind     string `json:"call_kind,omitempty"`  // "tool_iteration" | "phase_iteration" | "retry"
```
Update `writeLLMCallTrace` (and the `WriteLLMCallTrace` interface in `steps/services.go` + `service_adapters.go`) to take an explicit `iteration int` and `retryAttempt int` (or a small `TraceCounters` struct to avoid a params-explosion). Call sites map their real meaning:
- tool loop (`runner.go:337`): `iteration=iteration`, `retryAttempt=0`, `call_kind="tool_iteration"`.
- state-machine loop (`statemachineloop.go:177`): `iteration=sm.used[state]+1`, `call_kind="phase_iteration"`.
- retry site (`runner.go:265`): `retryAttempt=finalAttempt`, `call_kind="retry"`.

Back-compat: keep `retry_attempt` in the JSON (no external consumer reads it — verified across `server/` + `web/`), but it is now *correct* (not overloaded). Prefer the `TraceCounters` struct approach so the interface signature changes once, not per new counter.

### Test
Extend `runner_test.go`: assert a 3-iteration tool loop with no failure writes `iteration: 1,2,3`, `retry_attempt: 0`, and `call_kind: "tool_iteration"` for all 3 calls (guards against the exact mislabel that produced Finding 31).

## Issue 2 — Deterministic boundary widening (`steps/analyze.go`)

### Current state (verified)
`validateBoundaryCoverage` (`analyze.go:240–282`) already computes `uncovered []string` and the declared `roots`, then returns an error that `validateAnalyzeSpec` (`:312–315`) feeds back to the LLM — causing the full-spec regeneration seen in `call-008`→`call-009`.

The **frontend already solves the same problem deterministically** for the human path — `BoundaryResolutionControls.handleApprove` (`web/.../page.tsx`) derives `{module, root, repo_name, capabilities}` from each violating file's path:
```
root      = dir(file)                         // parent directory
module    = basename(root) (or "root")
repo_name = first path segment if multi-repo
capabilities = ["modify_existing","create_test","create_helper"]
```

### Change
Port that derivation server-side and run it **before** returning a coverage error. New function, pure/deterministic:
```go
// autoWidenBoundaries returns boundaries synthesized for uncovered files whose parent
// directory is NOT sensitive, plus the residual uncovered files that must still escalate.
func autoWidenBoundaries(uncovered []string, existing []ExecutionBoundary) (added []ExecutionBoundary, residual []string)
```
Wire into `validateAnalyzeSpec`:
1. **Refactor `validateBoundaryCoverage` signature** to `func validateBoundaryCoverage(analysis models.TaskAnalysis) (uncovered []string, err error)` — return the structured `uncovered` list directly instead of embedding it in an error string. Do NOT parse the error string to extract filenames.
2. `added, residual := autoWidenBoundaries(uncovered, analysisDraft.ExecutionBoundaries)`.
3. If `len(added) > 0`: append to `analysisDraft.ExecutionBoundaries` **and to the accepted `parsedJSON`** so the persisted contract reflects the widening (the model's other fields are untouched).
4. If `len(residual) > 0`: return the corrective error for the residual set only (existing round-trip) — or route to the human boundary-resolution pause for sensitive files.
5. If `residual` empty: accept (return nil). No second LLM call.

### Sensitivity policy (hard rule — promoted from Open Question)
Auto-widen is **denied** (file stays in `residual`) when its path matches any of: `.github/`, `deploy/`, `infra/`, `.ci/`, `**/secrets*`, `**/*.tfvars`, or is the repo-root `go.mod`/`go.sum`.

> **Critical constraint**: `autoWidenBoundaries` MUST NEVER synthesize a boundary with `root: "."` (or `root: ""` / `root: "./"`). The existing `validateBoundaryCoverage` treats `root == "."` as "covers everything" (`analyze.go:235-236`), so synthesizing it for a single file (e.g. `go.mod`) would silently grant repo-wide coverage — a massive security over-grant. Files at the repo root whose parent dir resolves to `.` MUST stay in `residual` and escalate to human/LLM.

Everything under `internal/`, `cmd/`, `pkg/` auto-widens. Keep this list in one named var (`sensitiveBoundaryPrefixes`) so it's reviewable and shared with the human-path logic if that later moves server-side.

### Determinism guarantees
- Sort `added` by `root` before appending (stable output).
- No model call in the auto-widen path → the accepted spec is a pure function of the last model output + the deterministic widening.

### Test
- `boundary_regression_test.go`: the existing case where `go.mod, cmd/sync/main.go` are uncovered — assert `cmd/sync/main.go` auto-widens (under `cmd/`) and, per policy, `go.mod` either widens via root `.` or escalates (pick one in the policy var and assert it); assert **no** second LLM iteration is consumed when residual is empty.
- Add a sensitive-path case (`deploy/prod.yaml` uncovered) asserting it stays in `residual` and does not get a synthesized boundary.

## Issue 3 — Timeline artifact contract linkage

### Current state (verified)
`worker.go:200-217` (inside the `engine.OnEvent` callback) writes to `artifacts/workflow_timeline.jsonl`:
```json
{"status":"running","step":"context_load","timestamp":"..."}
```
It also conditionally includes `"error"` when `event.Error != ""`. No linkage to `execution_units`/`execution_irs`, though those exist in the accepted spec and `task.Analysis` is in scope.

### Change
In `worker.go:200-217`, when `event.StepID` has a `code_` prefix (same `strings.HasPrefix` check already at `:168-170`), look up the matching `execution_unit` from `task.Analysis.ExecutionUnits` — the runtime maps `code_{role}_{idx}` → unit index; the frontend's `deriveImplementationItems` does the same mapping. Add the unit's `node_id`/`id` and `objective` to the `timelineEvent` map:
```json
{"status":"running","step":"code_backend_0","node_id":"sqlite_repository",
 "objective":"Thiết lập cấu trúc SQLite ...","timestamp":"..."}
```
Fields omitted for non-code steps (no null-spamming). Additive only — the JSONL stays append-only and existing readers ignore unknown keys.

### Test
Assert a `code_*` timeline record includes `node_id` + `objective`; assert a `context_load` record omits them and still parses.

## Rollout / risk
- **Issue 1** is internal observability; zero runtime-behavior change. Ship first.
- **Issue 2** changes accepted-spec content (adds boundaries) — highest care. Gate behind the sensitivity policy; the regression test is the safety net. This is the only change that alters what the agent is *authorized to touch*, so the sensitive-path denial list must be reviewed before merge.
- **Issue 3** is additive metadata; low risk. Ship anytime.

## Explicitly not designed (verification result)
No Prompt Compiler, Semantic Adapter, Execution IR (exists), Evidence Ledger, Decision Ledger, or Execution-Graph scheduler is designed here. Reason: refuted by artifacts or unobservable in an analyze-only run — see `proposal.md` verdict table. Revisit only against a log that actually captures `code`/`review`/`fix`.
