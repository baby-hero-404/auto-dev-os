# Tasks: Orchestrator Context Stability & Prompt Efficiency

## P0 — Critical

### Task 1.1: Implement FrozenContext Snapshot
> Links to: REQ-M01

**Acceptance Criteria:**
- [ ] Define `FrozenContext` struct in `server/pkg/models/`
- [ ] At end of Plan step, serialize `FrozenContext` from current `TaskAnalysis` and store as checkpoint artifact
- [ ] Add `LoadFrozenContext(ctx, taskID)` helper on the workspace manager
- [ ] Fallback: if `FrozenContext` checkpoint is missing, read from live `TaskAnalysis` (backward compat)

### Task 1.2: Wire FrozenContext into Prompt Assembly
> Links to: REQ-M01

**Acceptance Criteria:**
- [ ] `PromptAssembler.collect()` reads spec/boundary data from `FrozenContext` when available
- [ ] `code_backend.go` and `code_frontend.go` inject subtask context from frozen snapshot, not live `TaskAnalysis`
- [ ] `review.go` reads `TasksMD`, `AcceptanceCriteria`, `ExecutionBoundaries` from frozen snapshot
- [ ] Only `AffectedFiles` accumulation continues on live `TaskAnalysis`

### Task 1.3: Implement Context Cache in ContextLoad
> Links to: REQ-M02

**Acceptance Criteria:**
- [ ] Define `ContextCache` struct
- [ ] `ContextLoadStep.Execute()` calls `RetrieveContext()`, `GetRepoMap()`, `ScanDirectory()` and stores results in step output as `"context_cache"`
- [ ] `PromptAssembler.collect()` reads `context_cache` from `StepContext.Inputs["context_load"]` instead of calling `ContextEngine` methods directly
- [ ] Remove `ScanDirectory()` calls from `code_backend.go` and `code_frontend.go`

## P1 — High

### Task 2.1: Implement Typed Parse Errors
> Links to: REQ-M03

**Acceptance Criteria:**
- [ ] Define `ParseErrorKind` and `ClassifiedParseError` types in `llmrunner/`
- [ ] Refactor `ParseJSONMarkdown` to classify errors by detection heuristics (truncation, format, schema)
- [ ] Return `ClassifiedParseError` instead of generic `error`
- [ ] Log error kind alongside raw error message in `Runner.Run()`

### Task 2.2: Implement Error-Aware Retry
> Links to: REQ-M04

**Acceptance Criteria:**
- [ ] `Runner.Run()` switches retry strategy based on `ClassifiedParseError.Kind`
- [ ] `FormatError`: attempt `RepairJSONBrackets` + `SanitizeJSON` only (no LLM call)
- [ ] `TruncationError`: re-call LLM with truncation feedback message
- [ ] `SchemaError`: re-call with schema-specific correction
- [ ] `BusinessError` or unknown: full re-generation (current behavior)

### Task 2.3: Implement Prompt Deduplication
> Links to: REQ-M05

**Acceptance Criteria:**
- [ ] In `collect()`, when Execution Manifest JSON is injected for coding steps, skip markdown sections for `ProposalMD`, `SpecsMD`, `DesignMD`
- [ ] For non-coding steps (analyze, plan), continue injecting both (markdown is primary)
- [ ] Add test case verifying token reduction for coding step prompt assembly

## P2 — Medium

### Task 3.1: Add Budget Pruning Observability
> Links to: REQ-M06

**Acceptance Criteria:**
- [ ] `optimizeBudget()` logs each dropped section with `slog.Info()` including section name and token count
- [ ] Pruning summary is included in the LLM trace artifact via `WriteTrace`

### Task 3.2: Clean Up Dead Code
> Links to: REQ-R01, REQ-R02

**Acceptance Criteria:**
- [ ] Remove unused `branch` variable in `repoutil/paths.go:L29-32`
- [ ] Fix stale comment in `patch/applier.go:L327`
- [ ] Update `specs.md` status icons for `simplify-repo-path` spec from `❌` to `✅`

## P3 — Low

### Task 4.1: Decompose `collect()` Function
> Links to: REQ-M02

**Acceptance Criteria:**
- [ ] Extract rule loading into `collectRules()` method
- [ ] Extract context/snippet loading into `collectContext()` method
- [ ] Extract spec/manifest injection into `collectSpecs()` method
- [ ] `collect()` becomes an orchestrator calling these sub-collectors
- [ ] All existing tests pass without modification
