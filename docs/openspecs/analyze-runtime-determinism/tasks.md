# Tasks: Analyze-Step Runtime Determinism & Trace Clarity

> Ordered by risk (safest first). Scope = the 3 verified issues only. Findings 21–35 not covered here are excluded by design — see `proposal.md` verdict table.

## Phase 0: Verification (done — this is the deliverable that gated the rest)
- [x] Confirm the run reached only `context_load` → `analyze` (9 tool iterations) → spec-review pause (`workflow_timeline.jsonl`, event log, `task.json`).
- [x] Confirm analyze output is a structured contract with `execution_units` + `execution_irs` + DAG (`call-009/parsed.json`) → refutes F22/F25.
- [x] Confirm boundary + DAG + contract validation exists and fired (`analyze.go:240–328`, `call-008` rejection, `boundary_regression_test.go`) → refutes F32.
- [x] Confirm the "9 retries" are tool iterations mislabeled `retry_attempt` (`llm_trace.go` fed by `iteration`/`sm.used+1`/`finalAttempt`) → explains/downgrades F31.
- [x] Confirm timeline artifact is thin vs. the available contract → partially confirms F27.

## Phase 1: Trace iteration vs retry (REQ-001) — internal, ship first
- [x] **Mapped all call sites** via grep: `analyze.go` (tool loop), `statemachineloop.go:177` (phase loop), `runner.go:265` (retry) + `:337` (tool loop), `traceRecorderAdapter` (`service_adapters.go`). No others.
- [x] Added `Iteration int` + `CallKind string` (`call_kind,omitempty`) to the trace metadata struct in `llm_trace.go`; `RetryAttempt` retained and now only carries real retries.
- [x] Introduced `llmrunner.TraceCounters{Iteration, RetryAttempt, Kind}` + `TraceKind*` consts; threaded it through the `WriteTrace` func type, the `steps.TraceRecorder` interface, `traceRecorderAdapter`, and the concrete `writeLLMCallTrace` (single struct, no params-explosion).
- [x] Mapped call sites: `runner.go:337` → tool_iteration; `statemachineloop.go:177` → phase_iteration; `runner.go:265` → retry; `analyze.go` → tool_iteration.
- [x] `runner_test.go`: `TestRunner_Run_AgenticMode_TraceCountersAreIterations` asserts a no-failure tool loop writes ascending `iteration`, `retry_attempt: 0`, `call_kind: tool_iteration`.

## Phase 2: Deterministic boundary widening (REQ-002) — behavior change, most care
- [x] Split coverage computation into `collectUncoveredBoundaryFiles` (returns the structured `[]string`) + `boundaryFileIsCovered`; no error-string parsing.
- [x] Implemented `autoWidenBoundaries(uncovered, existing) (added, residual)` deriving `{module, root, capabilities}` from each file's parent dir; `added` sorted by `root`; same-dir files dedup to one boundary.
- [x] Defined `sensitiveBoundaryPrefixes` (`.github/`, `deploy/`, `infra/`, `.ci/`) + `isSensitiveBoundaryPath` (also `secrets*`, `*.tfvars`, root `go.mod`/`go.sum`); those stay in `residual`.
- [x] **Hard rule enforced**: `autoWidenBoundaries` never synthesizes `root: "."`/`""`/`"./"` — repo-root files (parent dir `.`) stay in `residual`. Unit-tested.
- [x] Wired into `validateAnalyzeSpec`: `added` appended to `analysisDraft` **and** reflected into `parsedJSON["execution_boundaries"]` (only that field changes); corrective error returned only for `residual`; accepts with no second LLM call when residual is empty.
- [x] Tests: `TestAutoWidenBoundaries` (happy/sensitive/root-denial/determinism/dedup); `analyze_step_test` + both `boundary_regression_test` cases re-pointed to sensitive/root paths (deploy/, repo-root `main.go`) to preserve their escalate/self-repair guarantees under the new behavior.
- [ ] Manual re-run of the `dde2df6c`-style task (needs a live LLM run) — not executed in this pass.

## Phase 3: Timeline artifact linkage (REQ-003) — additive, low risk
- [x] In `worker.go`'s timeline writer, for `isCodeStep` records, resolve the execution unit via new exported `llmrunner.UnitForStep` and add `node_id` + `objective`; omitted for non-code steps.
- [x] Test: `TestUnitForStep_MapsCodeStepsAndOmitsNonCode` — `code_backend_0`/`code_frontend_0` resolve to their units; `context_load`/`analyze` resolve to nil (fields omitted).

## Phase 4: Verification & docs
- [x] `go test ./...` under `server/` green; `go vet ./internal/orchestrator/...` clean.
- [x] `specs.md` status icons updated `❌`→`✅`.
- [ ] If a later run captures `code`/`review`/`fix`, open a **new** verification pass against that log before acting on report Findings 26/28/29/30/33/34/35.

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
