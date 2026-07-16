# Specs: Analyze-Step Runtime Determinism & Trace Clarity

> Scope is limited to the three issues that survived verification against run `dde2df6c-3ab5-462a-8513-0e2c4f559316`. See `proposal.md` for the full finding-by-finding verdict.

## Added Requirements

### REQ-001: LLM Trace Distinguishes Iteration from Retry
> ✅ Status: Shipped

**Scenario: tool-loop iteration is not a retry**
- WHEN the analyze (or code) tool-loop performs its Nth iteration without any preceding failure/retry
- THEN the written call trace MUST record that N as an `iteration` value
- AND it MUST NOT report that N as a failure-`retry_attempt` (today `metadata.json` shows `retry_attempt: 1..9` for 9 non-failing read-file iterations)
- AND a consumer reading the trace MUST be able to distinguish "9th tool iteration" from "9th retry after failure" from the fields alone.

**Scenario: a genuine retry**
- WHEN a step is actually retried after a failure
- THEN `retry_attempt` MUST reflect the real retry count, independent of the per-call `iteration`.

**Scenario: no regression for existing consumers**
- WHEN the trace schema changes
- THEN the change MUST be additive/back-compatible (no code path outside `llm_trace.go` reads `retry_attempt` today — verified), and existing per-call fields (`prompt_hash`, `cost_usd`, `latency_ms`, `model`, `call_number`) MUST be preserved.

### REQ-002: Deterministic Boundary Widening Before LLM Round-Trip
> ✅ Status: Shipped

**Scenario: uncovered files with non-sensitive parents are auto-covered**
- WHEN `validateBoundaryCoverage` detects affected/target files not covered by any `execution_boundary`, and every such file's parent directory is non-sensitive per the boundary policy
- THEN the analyze step MUST deterministically synthesize the missing boundary/boundaries from those files' directories (module/root derived from the path, mirroring the existing frontend `BoundaryResolutionControls` derivation) and accept the spec
- AND it MUST NOT feed the failure back to the LLM to regenerate the entire spec JSON for this case
- AND all other fields of the accepted spec MUST be byte-for-byte the model's last output (only `execution_boundaries` is augmented).

**Scenario: sensitive files still escalate**
- WHEN an uncovered file's parent directory matches the sensitive/infrastructure policy (e.g. deploy, CI, secrets, or the repo-root `go.mod` per policy)
- THEN the step MUST NOT auto-widen for that file
- AND it MUST surface the residual uncovered set (via the existing corrective round-trip or a human boundary-resolution prompt), never silently grant coverage.

**Scenario: root boundary never synthesized**
- WHEN an uncovered file resides at the repo root (its parent directory resolves to `.`)
- THEN `autoWidenBoundaries` MUST NOT synthesize a boundary with `root: "."` (or `""` or `"./"`)
- AND the file MUST stay in `residual` and escalate to human/LLM
- BECAUSE the existing `validateBoundaryCoverage` treats `root == "."` as "covers everything" (`analyze.go:235-236`), so synthesizing it for one file would silently grant repo-wide coverage.

**Scenario: determinism**
- WHEN the same spec output with the same uncovered non-sensitive set is validated twice
- THEN the synthesized boundaries MUST be identical (stable ordering, no LLM involvement), and the accepted spec MUST NOT depend on a second model call.

### REQ-003: Timeline Artifact Links to the Execution Contract
> ✅ Status: Shipped

**Scenario:**
- WHEN a `code_*` step transition is written to `artifacts/workflow_timeline.jsonl`
- THEN the record MUST include the matching `execution_unit`'s `node_id`/`id` and `objective`, in addition to the existing `{step, status, timestamp}`
- AND for non-`code` steps (context_load, analyze, merge) that have no execution unit, the record MUST remain valid with the enrichment fields omitted (not null-spammed)
- AND the enrichment MUST be additive — existing consumers of `{step,status,timestamp}` MUST continue to parse the records.

## Modified Requirements
- None (no existing requirement's contract changes; these are additive hardening).

## Removed Requirements
- None.

## Non-Requirements (verification-driven exclusions)
These are recorded so a future reader does not re-derive them from the v7.0 report:
- The system already emits a structured execution **contract** and **Execution IR** (`execution_units`, `execution_irs`) — no "OpenSpec-as-contract" or "planner compiler" requirement is created (refutes report F22/F25).
- The analyze step already runs **boundary + DAG/cost + contract-completeness validation** — no "add a validation pipeline" requirement is created (refutes report F32).
- Review/fix machine-readable output, evidence/decision ledgers, semantic adapter, and prompt analytics are **not specified** — no log evidence exists for those phases in this run (report F26/F28/F29/F30/F35).
