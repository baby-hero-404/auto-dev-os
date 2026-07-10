# Specs: Resilient Retry Pipeline

## Added Requirements

### REQ-001: Git Checkpoint on Step Completion
> ❌ Status: Not Started

**Scenario:**
- WHEN a workflow step (e.g. `code_backend_0`) completes successfully
- THEN the orchestrator creates a Git commit in the worktree with message `chore(auto-code-os): checkpoint [step_id]`
- AND the commit hash is stored in the checkpoint metadata

### REQ-002: Worktree Restore on Task Resume
> ❌ Status: Not Started

**Scenario:**
- WHEN a task is resumed from a previous checkpoint
- THEN the orchestrator executes `git checkout <commit_hash>` followed by `git reset --hard && git clean -fd`
- AND the existing `workspace_cache.db` is deleted
- AND `IndexWorkspace()` is called to rebuild the AST cache from restored source code

### REQ-003: Pre-Retry Worktree Reset
> ❌ Status: Not Started

**Scenario:**
- WHEN a coding step retry begins (attempt >= 2)
- THEN the system runs `git reset --hard HEAD && git clean -fd` inside the worktree container
- AND the worktree state matches the state before attempt 1
- AND if the reset fails, the step is aborted with error `"worktree corrupted, cannot retry"`

### REQ-004: Revert Error Logging
> ❌ Status: Not Started

**Scenario:**
- WHEN `ApplyPatch` fails and the revert attempt also fails
- THEN the revert error is logged with level `"error"` (not silently discarded)
- AND the log message contains the step ID and error details

### REQ-005: Auto-Populate AffectedFiles from Compiler Output
> ❌ Status: Not Started

**Scenario:**
- WHEN a compiler/test error contains file paths (e.g. `internal/model/commit.go:21:1: syntax error`)
- THEN the system parses file paths from the error output
- AND adds them to `analysis.AffectedFiles` before the retry LLM call
- AND `runner.go:54` subsequently injects file contents into the prompt

### REQ-006: Worktree-First File Resolution
> ❌ Status: Not Started

**Scenario:**
- WHEN `readAffectedFileContent` is called for a file in `AffectedFiles`
- THEN it first checks `Paths.Worktrees[role]` for the file
- AND falls back to `Paths.Main` only if the worktree lookup fails
- AND returns the actual content from the active worktree where code was written

### REQ-007: Full File Injection on Retry
> ❌ Status: Not Started

**Scenario:**
- WHEN a coding step retries after a compile/test error (attempt >= 2)
- THEN the retry instruction includes full file contents of all error files
- AND the content is formatted under a `### Current File Contents ###` header
- AND each file is wrapped in a fenced code block with its path

### REQ-008: Fresh Context on Retry
> ❌ Status: Not Started

**Scenario:**
- WHEN a coding step is retried
- THEN the PromptAssembler bypasses the static `context_cache`
- AND calls `ctxEngine.RetrieveContext` dynamically to get fresh semantic snippets
- AND non-retry steps continue to use the cache (no performance regression)

### REQ-009: Scoped Workspace Indexing
> ❌ Status: Not Started

**Scenario:**
- WHEN `IndexWorkspace` is called with an `AgentPathContext`
- THEN it scans only `pathCtx.PhysicalRoot()` (the active worktree)
- AND does NOT scan the entire `code/repos` directory
- AND no snippets from outside the worktree boundary are returned

### REQ-010: Drop Unresolvable Snippet Paths
> ❌ Status: Not Started

**Scenario:**
- WHEN `RetrieveContext` returns a snippet whose path fails `ToLogical` resolution
- THEN the snippet is dropped entirely
- AND the raw absolute host path is NOT used as a fallback
- AND no absolute host paths appear in any LLM prompt

### REQ-011: Sliding Window Error in Retries
> ❌ Status: Not Started

**Scenario:**
- WHEN a retry loop runs 3 attempts
- THEN each attempt's instruction contains only the base instruction + the latest error
- AND error text from previous attempts is NOT accumulated
- AND total instruction size does not grow beyond `base_size + one_error_block`

### REQ-012: Hunk Line Count Validation
> ❌ Status: Not Started

**Scenario:**
- WHEN `ValidateUnifiedDiff` receives a patch with hunk header `@@ -1,3 +1,19 @@`
- AND the actual hunk body contains 20 new-side lines (not 19)
- THEN a `ValidationError` with `IsFatal: true` is returned
- AND the error message includes expected vs actual counts
- AND the patch is NOT passed to `git apply`

### REQ-013: Auto-Switch to Search/Replace on Retry
> ❌ Status: Not Started

**Scenario:**
- WHEN a unified diff patch fails to apply on attempt 2 or later
- THEN the retry instruction switches to request Search/Replace format
- AND the instruction includes the `<<<<<<< SEARCH ... ======= ... >>>>>>> REPLACE` format example
- AND the `SearchReplaceApplier` in `engine.go` processes the response

### REQ-014: RAG Boost for Error Files
> ❌ Status: Not Started

**Scenario:**
- WHEN the PromptAssembler builds context for a retry step
- THEN file paths from compiler errors are prepended to the search query
- AND `maxSnippets` is raised from 4 to 8
- AND broken files mentioned in errors appear in the semantic context

## Modified Requirements

### REQ-M01: Patch Engine Validation Enhancement (extends 5.12)
> ❌ Status: Not Started

**Scenario:**
- WHEN `PatchValidator.Validate()` is called on a unified diff
- THEN it runs both the existing `oldStart > fileLines` check AND the new `ValidateHunkCounts` check
- AND both validations must pass before the patch is applied

## Removed Requirements

- REQ-R01: Silent revert error discard (`_, _ = r.RunSandboxStepInWorktree(...)`) — replaced by explicit error logging (REQ-004)
