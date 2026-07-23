# Tasks: Orchestrator Context Stability & Prompt Efficiency

## P0 — Critical

### Task 1.1: Implement FrozenContext Snapshot
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] Define `FrozenContext` struct in `server/pkg/models/`
- [x] At end of Plan step, serialize `FrozenContext` from current `TaskAnalysis` and store as checkpoint artifact
- [x] Add `LoadFrozenContext(ctx, taskID)` helper on the workspace manager
- [x] Fallback: if `FrozenContext` checkpoint is missing, read from live `TaskAnalysis` (backward compat)

### Task 1.2: Wire FrozenContext into Prompt Assembly
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] `PromptAssembler.collect()` reads spec/boundary data from `FrozenContext` when available
- [x] `code_backend.go` and `code_frontend.go` inject subtask context from frozen snapshot, not live `TaskAnalysis`
- [x] `review.go` reads `TasksMD`, `AcceptanceCriteria`, `ExecutionBoundaries` from frozen snapshot
- [x] Only `AffectedFiles` accumulation continues on live `TaskAnalysis`

### Task 1.3: Implement Context Cache in ContextLoad
> Links to: REQ-M02

**Acceptance Criteria:**
- [x] Define `ContextCache` struct
- [x] `ContextLoadStep.Execute()` calls `RetrieveContext()`, `GetRepoMap()`, `ScanDirectory()` and stores results in step output as `"context_cache"`
- [x] `PromptAssembler.collect()` reads `context_cache` from `StepContext.Inputs["context_load"]` instead of calling `ContextEngine` methods directly
- [x] Remove `ScanDirectory()` calls from `code_backend.go` and `code_frontend.go`

## P1 — High

### Task 2.1: Implement Typed Parse Errors
> Links to: REQ-M03

**Acceptance Criteria:**
- [x] Define `ParseErrorKind` and `ClassifiedParseError` types in `llmrunner/`
- [x] Refactor `ParseJSONMarkdown` to classify errors by detection heuristics (truncation, format, schema)
- [x] Return `ClassifiedParseError` instead of generic `error`
- [x] Log error kind alongside raw error message in `Runner.Run()`

### Task 2.2: Implement Error-Aware Retry
> Links to: REQ-M04

**Acceptance Criteria:**
- [x] `Runner.Run()` switches retry strategy based on `ClassifiedParseError.Kind`
- [x] `FormatError`: attempt `RepairJSONBrackets` + `SanitizeJSON` only (no LLM call)
- [x] `TruncationError`: re-call LLM with truncation feedback message
- [x] `SchemaError`: re-call with schema-specific correction
- [x] `BusinessError` or unknown: full re-generation (current behavior)

### Task 2.3: Implement Prompt Deduplication
> Links to: REQ-M05

**Acceptance Criteria:**
- [x] In `collect()`, when Execution Manifest JSON is injected for coding steps, skip markdown sections for `ProposalMD`, `SpecsMD`, `DesignMD`
- [x] For non-coding steps (analyze, plan), continue injecting both (markdown is primary)
- [x] Add test case verifying token reduction for coding step prompt assembly

## P2 — Medium

### Task 3.1: Add Budget Pruning Observability
> Links to: REQ-M06

**Acceptance Criteria:**
- [x] `optimizeBudget()` logs each dropped section with `slog.Info()` including section name and token count
- [x] Pruning summary is included in the LLM trace artifact via `WriteTrace`

### Task 3.2: Clean Up Dead Code
> Links to: REQ-R01, REQ-R02

**Acceptance Criteria:**
- [x] Remove unused `branch` variable in `repoutil/paths.go:L29-32`
- [x] Fix stale comment in `patch/applier.go:L327`
- [x] Update `specs.md` status icons for `simplify-repo-path` spec from `❌` to `✅`

## P3 — Low

### Task 4.1: Decompose `collect()` Function
> Links to: REQ-M02

**Acceptance Criteria:**
- [x] Extract rule loading into `collectRules()` method
- [x] Extract context/snippet loading into `collectContext()` method
- [x] Extract spec/manifest injection into `collectSpecs()` method
- [x] `collect()` becomes an orchestrator calling these sub-collectors
- [x] All existing tests pass without modification

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
