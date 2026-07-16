# Proposal: Analyze-Step Runtime Determinism & Trace Clarity

## Why
This set exists because an external report — *"Architecture vs Runtime Gap Analysis" (v7.0, Findings 21–35)* — was written **by inference from a single execution log** and asked to be verified before acting on it. It was. The verification is the primary deliverable of this proposal; the fixes are scoped to only what the evidence actually supports.

### What the evidence is
- **Log**: `server/.data/logs/dde2df6c-3ab5-462a-8513-0e2c4f559316.jsonl` (21 events)
- **Workspace**: `server/.data/workspaces/dde2df6c-3ab5-462a-8513-0e2c4f559316/`
- **What the run actually did** (from `artifacts/workflow_timeline.jsonl` + event log): `context_load` (success) → `analyze` (9 tool-loop iterations on `gemini-3.5-flash`) → **paused for human spec review**. `task.json` confirms `status: spec_review`, `spec_status: pending_review`.
- **Critical scope fact**: *no `plan`, `code`, `review`, `fix`, or `merge` step ran in this log.* There are zero coding iterations, zero review→fix cycles, zero diffs, zero workflow-level retries. Any report finding about those phases is, against this evidence, **unverifiable** — not confirmed.

### Verification verdict (report Findings 21–35)
Grouped by what the artifacts show. Evidence citations are to files inspected during verification.

| Finding | Verdict | Evidence |
|--------|---------|----------|
| **F22** — "OpenSpec behaves like documentation, not a contract" | **Refuted** | `call-009/parsed.json` is a structured contract: `execution_units` (id, objective, `target_files`, `dependencies`, `execution_profile{agent,skills}`, `constraints{estimated_tokens,max_files,max_risk}`), `execution_boundaries`, `acceptance_criteria`, DAG via `tasks[].depends_on`. It is contract **and** prose (`proposal_md`/`design_md`), not prose only. |
| **F25** — "Planner output not compiled; missing Execution IR" | **Refuted** | An `execution_irs` array exists in the analyze output, each entry with `node_id`, `intent{capability,operation}`, `acceptance[]`, `constraints[]`, `budget{discovery,implementation,validation}`. The IR the report says is "missing" is emitted. |
| **F32** — "Prompt Compiler lacks validation (path/boundary/tool/semantic)" | **Refuted** | `steps/analyze.go:284–328` runs a real validation pipeline every tool-loop turn: contract field-completeness, **boundary coverage** (`validateBoundaryCoverage`, `analyze.go:240–282`), and DAG/cost (`policy.ValidateDAG`). It **fired in this very run** — `call-008/prompt.md` tail contains the machine-generated rejection *"Boundary coverage validation failed: … do not cover: go.mod, cmd/sync/main.go"*. Regression test: `orchestrator/boundary_regression_test.go`. |
| **F31** — "Retry appears stateless; repeated reasoning; retry loops" | **Misdiagnosed** | The 9 calls are **tool-loop iterations** (read `model.go`, `go.mod`, `README.md`, `client.go`, then draft spec), not failure retries — see per-call `output.md` and event log *"StepAnalyze Iteration 1..9"*. They are mislabeled `retry_attempt: 1..9` in `logs/llm/call-00N/metadata.json`. **This mislabel is almost certainly what produced the "retry loop" reading.** → driver for REQ-001. |
| **F27** — "Execution artifacts are too weak (no node/goal/status)" | **Partially confirmed** | The rich per-node semantics *exist* in the contract (`execution_irs`, `execution_units`), but the runtime **timeline artifact** `artifacts/workflow_timeline.jsonl` records only `{step,status,timestamp}` and never references `node_id`/`goal`/`acceptance`. The gap is not "artifacts lack the data" — it's "the runtime timeline doesn't link to the contract it already has." → driver for REQ-003. |
| **F21, F23, F24, F33, F34** — arch/runtime divergence; primitives underutilized; prompt is a universal container; step- vs graph-oriented; business vs execution state | **Not supported by this evidence** | The single artifact these lean on (context growth) is the ordinary tool-loop appending tool results (`prompt_tokens` 12.6k→28k across 9 turns). A DAG (`depends_on`), an execution IR, and a `DISCOVERY/IMPLEMENTATION/VALIDATION` state machine (`llmrunner/statemachineloop.go`) already exist; none of the *execution* phases these findings describe ran in this log to observe divergence. Aspirational, not demonstrated. |
| **F26, F28, F29, F30, F35** — semantic adapter; machine-readable review output; evidence ledger; decision ledger; prompt-analytics loop | **Unverifiable here** | Require `review`/`fix` cycles that never ran in this log. Note: F28 is likely **already addressed** — git history shows *"harden the review-fix interface with explicit typed findings"* and `ReportFindings` typed output already exists. F35's raw data (per-call `prompt_tokens`/`cost_usd`/`latency_ms`/`prompt_hash`) is already captured; only a consumer is absent. |

**Bottom line**: the report's central thesis — *"the runtime owns infrastructure, the prompt owns execution semantics; structured primitives are unused"* — is **largely contradicted** by this run, which demonstrates a structured contract, an execution IR, a dependency DAG, and a working deterministic validator. The report author appears not to have opened the `parsed.json` / `execution_irs` artifacts. **Three narrow, real issues survived verification** and are the entire scope of this set.

## What Changes

### Issue 1: `retry_attempt` conflates tool-loop iteration with failure-retry (observability)
`orchestrator/llm_trace.go:80` emits a single JSON key `retry_attempt`, but its callers pass three different things: the tool-loop **iteration** (`runner.go:337`), the state-machine phase iteration (`statemachineloop.go:177`, `sm.used[currentState]+1`), and a genuine retry count (`runner.go:265`, `finalAttempt`). A reader cannot tell "9th read-file iteration" from "9th failed retry" — the exact confusion that drove Finding 31. Split the concept: always emit an accurate `iteration`, and reserve `retry_attempt` for real retries (or add an explicit `call_kind`). No UI/analytics consumer reads the key today (verified), so this is a safe internal change.

### Issue 2: Boundary-coverage failure forces full-spec regeneration (determinism + cost)
When `validateBoundaryCoverage` finds uncovered files, it returns an error string that is fed back to the LLM (`analyze.go:312–315`), which then **re-emits the entire spec JSON** to add a couple of boundary entries — observed as `call-007` (full spec, 5669 output tokens) → boundary rejection → `call-008`/`call-009` (full spec regenerated, 5790 output tokens, ~$0.016). This (a) burns an extra LLM round-trip and (b) risks the model silently altering *other* fields during regeneration (non-determinism). The validator already computes the exact `uncovered` set and declared `roots` (`analyze.go:250–278`) — the same information the **frontend already uses to auto-derive boundaries deterministically** (`BoundaryResolutionControls.handleApprove` in `page.tsx`). Move that deterministic widening server-side: auto-add boundaries for uncovered files whose parent directory is non-sensitive, and only round-trip to the LLM for the residual (infrastructure/security-sensitive) cases.

### Issue 3: Runtime timeline artifact doesn't reference the execution contract (traceability)
`artifacts/workflow_timeline.jsonl` records `{step,status,timestamp}` only. For `code_*` steps it should also carry the `node_id` / `objective` from the matching `execution_unit` (which already exists in the accepted spec), so the timeline is replayable/auditable against the contract rather than being a bare status log. Low-risk, additive fields.

## Capabilities

### New Capabilities
- `deterministic-boundary-widening`: server-side auto-coverage of uncovered affected/target files from their directories, before falling back to an LLM round-trip.

### Modified Capabilities
- LLM call trace: `iteration` vs `retry_attempt` disambiguated (accurate, non-overloaded).
- Workflow timeline artifact: enriched with `node_id`/`objective` for contract-linked steps.

### Removed Capabilities
- None.

## Impact

| Area | Files Affected |
|------|----------------|
| Trace schema | `server/internal/orchestrator/llm_trace.go` |
| Trace call sites | `server/internal/orchestrator/llmrunner/runner.go`, `.../statemachineloop.go`, `server/internal/orchestrator/service_adapters.go`, `server/internal/orchestrator/steps/services.go` |
| Boundary auto-widen | `server/internal/orchestrator/steps/analyze.go` (`validateBoundaryCoverage`, `validateAnalyzeSpec`) |
| Timeline artifact | writer of `artifacts/workflow_timeline.jsonl` (workflow status recorder) |
| Tests | `orchestrator/boundary_regression_test.go`, `steps/analyze_step_test.go`, `llmrunner/runner_test.go` |

## Out of Scope (explicitly, per verification)
Findings **21, 22, 23, 24, 25, 26, 28, 29, 30, 32, 33, 34, 35** are **not** actioned here: they are either refuted by the artifacts (22/25/32), unobservable in an analyze-only log (26/28/29/30/33/34), or aspirational reframings not supported by evidence (21/23/24/35). If a future run captures `code`/`review`/`fix` phases, re-open verification against *that* log before specifying work for them — do not build from the v7.0 report's inference alone.

## Open Questions
- **Sensitivity policy for auto-widening (Issue 2)**: The core policy is now a hard rule (see `design.md`): `autoWidenBoundaries` MUST NEVER synthesize `root: "."` — files at the repo root always escalate. The `sensitiveBoundaryPrefixes` list (`.github/`, `deploy/`, `infra/`, `.ci/`, `**/secrets*`, `**/*.tfvars`) is the default denial set; it may need extension per-project but the root-boundary prohibition is absolute.
- **Back-compat for `retry_attempt` (Issue 1)**: keep the key (populated correctly) vs. add `iteration` + deprecate. No consumer reads it today, so either is safe; design.md proposes additive `iteration` + corrected `retry_attempt`.
