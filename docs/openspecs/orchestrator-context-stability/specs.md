# Specs: Orchestrator Context Stability & Prompt Efficiency

## Modified Requirements

### REQ-M01: Frozen Context at Plan Boundary
> ✅ Status: Completed

**Scenario:**
- WHEN the Plan step completes successfully
- THEN a `FrozenContext` snapshot is persisted as a checkpoint artifact
- AND all subsequent coding/review/fix steps read spec data from the frozen snapshot
- AND only `AffectedFiles` accumulation is allowed on the live `TaskAnalysis`

### REQ-M02: Context Cache Layer
> ✅ Status: Completed

**Scenario:**
- WHEN `ContextLoadStep` completes
- THEN `RetrieveContext()` results, `RepoMap`, and `ScanDirectory()` tree are cached in the step output artifact
- AND `PromptAssembler.collect()` reads from cache instead of re-querying `ContextEngine`
- AND no redundant `ScanDirectory()` calls are made in coding steps

### REQ-M03: Typed Parse Error Classification
> ✅ Status: Completed

**Scenario:**
- WHEN `ParseJSONMarkdown` fails to parse LLM output
- THEN the error is classified as `FormatError`, `TruncationError`, `SchemaError`, or `BusinessError`
- AND the classified error type is logged alongside the raw error message
- AND the error type is returned to the caller for retry strategy selection

### REQ-M04: Error-Aware Retry Strategy
> ✅ Status: Completed

**Scenario:**
- WHEN a `FormatError` occurs
- THEN only local JSON repair is attempted (no LLM re-call)
- WHEN a `TruncationError` occurs
- THEN the LLM is re-called with truncation feedback
- WHEN a `SchemaError` occurs
- THEN the LLM is re-called with schema correction feedback only
- WHEN a `BusinessError` occurs
- THEN a full LLM re-generation is performed

### REQ-M05: Prompt Content Deduplication
> ✅ Status: Completed

**Scenario:**
- WHEN the Execution Manifest JSON is injected for coding steps
- THEN the equivalent markdown sections (ProposalMD, SpecsMD, DesignMD) are NOT injected
- AND the total prompt token count is reduced by the deduplicated content

### REQ-M06: Budget Pruning Observability
> ✅ Status: Completed

**Scenario:**
- WHEN `optimizeBudget()` drops a mutable section to meet the token limit
- THEN a log entry is emitted with the section name, token count, and remaining budget
- AND the pruning info is included in the LLM trace artifact

## Removed Requirements

### REQ-R01: Direct TaskAnalysis Spec Mutation During Coding
- Coding steps should no longer mutate spec/boundary fields on `TaskAnalysis`. Only `AffectedFiles` accumulation is permitted.

### REQ-R02: Per-Step ScanDirectory
- Individual coding steps should no longer call `ScanDirectory()`. They should read from the cached tree.
