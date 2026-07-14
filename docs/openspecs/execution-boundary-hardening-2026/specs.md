# Specs: Execution Boundary & Target Resolution Hardening

## Added Requirements

### REQ-001: Boundary Coverage Validation at Analyze
> ❌ Status: Not Started

**Scenario: uncovered file is repaired in-loop**
- WHEN the analyze LLM returns output where any `affected_files[].file` (e.g. `cmd/zentao-sync/main.go`) is not under any `execution_boundaries[].root` (e.g. only `internal/`)
- THEN validation rejects the response with an error naming each uncovered file and the declared roots
- AND the corrective error is fed back through the tool-loop `Validate` hook for the LLM to repair
- AND a subsequent response whose boundaries cover all affected files passes validation.

**Scenario: budget exhausted with coverage still broken**
- WHEN the analyze tool-loop budget exhausts and coverage is still violated
- THEN the analyze step fails with an error listing the uncovered files
- AND no `code_backend_*` step is ever started for this job.

### REQ-002: Per-Unit Target Files
> ❌ Status: Not Started

**Scenario: schema requires per-unit targets**
- WHEN analyze output contains an `execution_units[]` entry with a missing or empty `target_files` list
- THEN validation rejects the response with an error naming the unit
- AND each `target_files[]` path must also satisfy REQ-001 coverage.

**Scenario: coding step prompt is scoped to its unit**
- WHEN `code_backend_1` (mapped via subtask index to unit `api-clients`) has its initial messages built
- THEN its "Workspace Affected Files" section contains exactly `api-clients.target_files`
- AND contains no file belonging only to another unit (e.g. `init-core`'s `sqlite.go`).

### REQ-003: Intent Resolution Independent of Capability Language
> ❌ Status: Not Started

**Scenario: natural-language capability resolves via target_files**
- WHEN an ExecutionIR's `intent.capability` is a natural-language sentence (e.g. `"Thiết lập cấu trúc dự án và SQLite"`) and its unit declares non-empty `target_files`
- THEN `ResolveIntent` returns those `target_files` as the node's resolved targets
- AND no syllable-token matching is attempted against the sentence.

**Scenario: identifier capability keeps existing behavior**
- WHEN `intent.capability` is identifier-style (e.g. `"UserRepository"`) and no `target_files` are declared
- THEN token-based matching against candidates behaves exactly as today.

**Scenario: structured failure names the failed strategy**
- WHEN neither `target_files` nor token matching yields any path
- THEN the returned `IntentResolutionError` states which strategies were attempted and why each failed.

### REQ-004: Tool-Loop No-Progress Safeguard
> ❌ Status: Not Started

**Scenario: no-op edit is rejected and not counted**
- WHEN the LLM issues `search_replace` whose `search` equals its `replace`
- THEN the call is not executed; a corrective tool result explains the edit is a no-op
- AND the call is not added to `EditsApplied`
- AND a loop that exhausts its budget with only no-op edits is NOT reported as a salvageable partial result.

**Scenario: repeated identical successful read-only call**
- WHEN the LLM repeats a read-only call (e.g. `list_files` on `"."`) with arguments identical to an earlier successful call in the same run
- THEN the tool is not re-executed; the tool result instructs the model to make progress (write within its boundary, or explain in its summary why it cannot)
- AND the behavior is identical in `RunToolLoop` and `runStateMachine`.

### REQ-005: Structural-Failure Retry Policy
> ❌ Status: Not Started

**Scenario: zero-edit failure is not blindly retried**
- WHEN a coding/fix step fails having applied zero workspace edits
- THEN the failure is classified structural (`ErrNoProgress`)
- AND the worker does not consume the remaining `maxRetries` attempts for it (at most one total re-attempt, not two).

**Scenario: retry prompt carries the prior blocker**
- WHEN a retry of a failed coding/fix step is attempted
- THEN the new attempt's instruction includes the previous attempt's terminal error text (e.g. the execution-boundary-violation message)
- AND the transcript of the new attempt shows the model was given that error before its first tool call.

## Modified Requirements

### REQ-M01: Salvage Reporting Accuracy
> ❌ Status: Not Started

**Scenario:**
- WHEN a tool loop exhausts its budget after N successful edit calls of which M were no-ops
- THEN the salvage log line reports N−M edits
- AND `Partial` is true only if N−M > 0.

## Removed Requirements
- REQ-R01: Unconditional `maxRetries` (3×) workflow retry for coding/fix step failures that made no workspace progress.
- REQ-R02: Task-wide `affected_files` list injected verbatim into every `code_backend_N` prompt.
