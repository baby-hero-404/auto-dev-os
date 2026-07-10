# Proposal: Resilient Retry Pipeline

## Why

Through debugging workspace `4c19a5f1-2f4f-4012-8f7d-9e8a8569e317` (see `docs/report/git_parse_debug_report.md`), 8 confirmed architectural gaps were identified in the patch application, retry, and context pipeline. These gaps create a **cascading failure loop**:

1. **Silent Revert Corruption (Gap G):** When `ApplyPatch` fails, the revert attempt at `applier.go:307` silently discards errors (`_, _ = r.RunSandboxStepInWorktree(...)`). The `patch --batch` fallback applies hunks individually, leaving files partially modified (truncated, missing closing braces). All subsequent retry attempts operate on corrupted files.

2. **Zero File Context During Retries (Gap A+D):** `AffectedFiles` is only populated after step completion (`code_backend.go:373-393`), so during retries it's `null`. Additionally, `readAffectedFileContent` resolves paths against `Paths.Main` (empty checkout with only `.git/`) instead of `Paths.Worktrees["backend"]`. The LLM receives zero file contents and must hallucinate context lines.

3. **Stale Semantic Context (Gap E):** `ContextLoadStep` builds a static `context_cache` once. In `builder.go:701`, if cache exists, the assembler skips `ctxEngine.RetrieveContext`. Retry attempts use snippets from before any code was written.

4. **Host Path Leakage (Gap F):** `IndexWorkspace` scans the entire `code/repos` directory. When `ToLogical` fails for out-of-worktree paths, the fallback at `provider.go:434` leaks raw absolute host paths into the LLM prompt.

5. **Instruction Token Bloat (Gap H):** Each retry appends full error text to the instruction string. By attempt 3, accumulated errors consume the fixed token budget, leaving less room for actual code context. Pattern exists in `code_backend.go`, `code_frontend.go`, and `fix.go`.

6. **Missing Hunk Validation:** `ValidateUnifiedDiff` only checks `oldStart > len(fileLines)`. It does NOT validate that hunk header line counts match actual hunk body lines — the exact bug causing `malformed patch at line 44`.

7. **No Strategy Auto-Switch:** `SearchReplaceApplier` exists in `engine.go` but is only selected via explicit config. It's never auto-selected when unified diff retries fail repeatedly.

8. **RAG Scoring Mismatch (Gap B+C):** RAG keyword scoring favors well-structured files. Broken files score poorly and get excluded by `maxSnippets = 4`.

## What Changes

### Issue 1: Worktree Integrity via Git Checkpoints
- Create Git commits at each successful step completion as checkpoint snapshots.
- On task resume/rollback, `git checkout <commit>` + delete `workspace_cache.db` + re-index.
- Run `git reset --hard HEAD && git clean -fd` before each retry attempt (attempt >= 2).
- Log revert errors at `applier.go:307` and `:398` instead of discarding.

### Issue 2: Retry Context Reconstruction
- Parse compiler/test error output to extract file paths automatically.
- Update `analysis.AffectedFiles` dynamically before each retry LLM call.
- Refactor `readAffectedFileContent` to check `Paths.Worktrees[role]` before `Paths.Main`.
- Inject full file contents of error files under `### Current File Contents ###` in retry instructions.

### Issue 3: Fresh Context Pipeline
- Bypass static `context_cache` for coding retry steps; call `RetrieveContext` dynamically.
- Scope `IndexWorkspace` to `AgentPathContext.PhysicalRoot()` instead of entire `code/repos`.
- Drop snippets with failed `ToLogical` resolution instead of leaking absolute paths.

### Issue 4: Retry Optimization
- Replace cumulative error appending with sliding-window (only latest error).
- Add `ValidateHunkCounts` to catch hunk line count mismatches pre-apply.
- Auto-switch instruction to Search/Replace format after 2 unified diff failures.
- Boost RAG query with error file paths and raise `maxSnippets` from 4 to 8 on retry.

## Capabilities

### New Capabilities
- Git checkpoint commits at step boundaries (macro-level worktree snapshots)
- Pre-retry `git reset --hard HEAD` guard (micro-level worktree reset)
- Automatic `AffectedFiles` population from compiler error output
- Hunk line count validation (`ValidateHunkCounts`)
- Retry-aware context refresh (bypass stale cache)
- Auto-strategy switch (unified diff → search/replace on repeated failure)

### Modified Capabilities
- `ApplyPatch` revert: logs errors instead of silent discard
- `readAffectedFileContent`: worktree-first path resolution
- `PromptAssembler.Build`: retry-aware cache bypass
- `IndexWorkspace`/`RetrieveContext`: scoped to `AgentPathContext`
- Retry instruction: sliding-window error (not cumulative)
- `builder.go`: raised `maxSnippets` and boosted error file scoring for retries

### Removed Capabilities
- Silent revert error discard pattern (`_, _ = ...`)
- Static-only context cache for coding steps

## Impact

| Area | Files Affected |
|------|----------------|
| Worktree Integrity | `server/internal/orchestrator/patch/applier.go`, `server/internal/orchestrator/worker.go` |
| Pre-Retry Reset | `server/internal/orchestrator/steps/code_backend.go`, `server/internal/orchestrator/steps/code_frontend.go`, `server/internal/orchestrator/steps/fix.go` |
| Context Reconstruction | `server/internal/orchestrator/sandbox.go`, `server/internal/orchestrator/llmrunner/runner.go` |
| Fresh Context | `server/internal/prompts/builder.go`, `server/internal/context/provider/provider.go` |
| Patch Validation | `server/internal/orchestrator/patch/validator.go` |
| Strategy Switch | `server/internal/orchestrator/patch/engine.go` |
