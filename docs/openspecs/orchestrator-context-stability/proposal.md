# Proposal: Orchestrator Context Stability & Prompt Efficiency

## Why

Through systematic verification of 20 architectural hypotheses (see `docs/reports/verified_architecture_findings.md`), 6 confirmed issues were identified in the current orchestration pipeline. These issues cause:

1. **Context Drift**: `TaskAnalysis` is mutated mid-workflow (e.g., `AffectedFiles` added during coding steps), meaning later agents see different context than earlier agents. This violates the immutability principle of execution contracts.
2. **Redundant Computation**: Repository structure, semantic context, and repo maps are recomputed on **every LLM call** instead of being cached once during ContextLoad. This wastes latency and tokens.
3. **Non-diagnostic Parse Failures**: The JSON parser chains 6 fallback strategies with no error classification. When parsing fails, the root cause (truncation, schema mismatch, format error) is invisible.
4. **Expensive Retry Loops**: Both parse failures and patch apply failures trigger full LLM re-invocation instead of targeted surgical repair.
5. **Dual Prompt Representation**: The same spec data is injected as both markdown AND JSON into prompts, wasting tokens.
6. **Invisible Budget Pruning**: When prompt sections are dropped to meet token budgets, no observability is provided.

## What Changes

### Issue 1: Context Immutability
- Freeze `TaskAnalysis` at Plan step completion into a `FrozenContext` snapshot.
- All subsequent steps read from the frozen snapshot, not the live `TaskAnalysis`.
- Only the `AffectedFiles` accumulator operates on the live object; spec/boundary data is immutable.

### Issue 2: Context Caching
- Cache `RetrieveContext()` results and `RepoMap` output in the ContextLoad step artifact.
- Subsequent prompt assemblies read from cache instead of re-querying the ContextEngine.
- Cache `ScanDirectory()` result and share across coding steps.

### Issue 3: Structured Parse Errors
- Split `ParseJSONMarkdown` into layered stages with typed error returns.
- Classify failures as: `FormatError`, `TruncationError`, `SchemaError`, `BusinessError`.
- Log classified error type alongside raw error.

### Issue 4: Targeted Retry Strategy
- For `FormatError`: attempt JSON repair only (no LLM re-call).
- For `TruncationError`: re-call LLM with `max_tokens` increase hint.
- For `SchemaError`: re-call with schema feedback only.
- Full re-generation only for `BusinessError`.

### Issue 5: Deduplicate Prompt Content
- When Execution Manifest JSON is injected, skip the equivalent markdown sections.
- Prefer JSON for coding steps; prefer markdown for analyze/plan steps.

### Issue 6: Budget Pruning Observability
- Log which sections are dropped, their token count, and the remaining budget.
- Include pruning info in the LLM trace artifact.

## Capabilities

### New Capabilities
- `FrozenContext` snapshot mechanism for workflow execution isolation
- Context cache layer between ContextLoad and PromptAssembler
- Typed parse error classification (`FormatError`, `TruncationError`, `SchemaError`)
- Budget pruning observability (logging + trace)

### Modified Capabilities
- `PromptAssembler.collect()` reads from cache instead of live queries
- `ParseJSONMarkdown` returns typed errors with classification
- `llmrunner.Runner.Run()` retry strategy is error-type-aware
- `optimizeBudget()` logs dropped sections

### Removed Capabilities
- Direct `TaskAnalysis` mutation during coding steps (replaced with accumulator pattern)
- Per-step `ScanDirectory()` calls (replaced with cached tree)

## Impact

| Area | Files Affected |
|------|----------------|
| Context Freeze | `server/internal/orchestrator/steps/plan.go`, `server/internal/orchestrator/worker.go` |
| Context Cache | `server/internal/orchestrator/steps/context_load.go`, `server/internal/prompts/builder.go`, `server/internal/prompts/assembler.go` |
| Parse Errors | `server/internal/orchestrator/llmrunner/json.go`, `server/internal/orchestrator/llmrunner/runner.go` |
| Targeted Retry | `server/internal/orchestrator/llmrunner/runner.go`, `server/internal/orchestrator/steps/code_backend.go`, `server/internal/orchestrator/steps/code_frontend.go` |
| Prompt Dedup | `server/internal/prompts/builder.go` |
| Budget Observability | `server/internal/prompts/builder.go` |
| Dead Code | `server/internal/orchestrator/repoutil/paths.go`, `server/internal/orchestrator/patch/applier.go` |
