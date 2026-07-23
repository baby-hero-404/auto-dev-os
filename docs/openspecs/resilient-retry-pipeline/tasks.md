# Tasks: Resilient Retry Pipeline

## P0 — Critical

### Task 1.1: Log Revert Errors in ApplyPatch
> Links to: REQ-004

**Acceptance Criteria:**
- [x] `applier.go:307` — replace `_, _ =` with error capture and log at `"error"` level
- [x] `applier.go:398` — same fix for multi-repo path
- [x] Log message includes step ID and full error text
- [x] Unit test: simulate failed revert, verify error is logged

### Task 1.2: Pre-Retry Worktree Reset
> Links to: REQ-003

**Acceptance Criteria:**
- [x] `code_backend.go` retry loop runs `git reset --hard HEAD && git clean -fd` before attempt >= 2
- [x] `code_frontend.go` — same change
- [x] `fix.go` — same change
- [x] If reset fails, step aborts with `"worktree corrupted"` error
- [x] Unit test: simulate corrupted worktree, verify reset restores clean state

### Task 1.3: Git Checkpoint Commits on Step Completion
> Links to: REQ-001

**Acceptance Criteria:**
- [x] Worker creates `git add . && git commit` after successful step
- [x] Commit message format: `chore(auto-code-os): checkpoint [step_id]`
- [x] Commit hash stored in checkpoint metadata (`Checkpoint.CommitHash`)
- [x] Integration test: verify checkpoint commit exists in worktree git log

### Task 1.4: Worktree Restore on Task Resume
> Links to: REQ-002

**Acceptance Criteria:**
- [x] Resume logic executes `git checkout <commit_hash>` + `git reset --hard && git clean -fd`
- [x] `workspace_cache.db` is deleted before re-indexing
- [x] `IndexWorkspace()` is called to rebuild fresh cache
- [x] Integration test: resume from checkpoint, verify file state matches checkpoint

### Task 1.5: Parse Compiler Output into AffectedFiles
> Links to: REQ-005

**Acceptance Criteria:**
- [x] `parseCompilerErrorFiles` function extracts file paths from Go compiler output
- [x] Parsed files are added to `analysis.AffectedFiles` before retry LLM call
- [x] Works for patterns like `file.go:line:col: error message`
- [x] Unit test: parse `"internal/model/commit.go:21:1: syntax error"` → `["internal/model/commit.go"]`

### Task 1.6: Worktree-First File Resolution
> Links to: REQ-006

**Acceptance Criteria:**
- [x] `readAffectedFileContent` checks `Paths.Worktrees[role]` before `Paths.Main`
- [x] Falls back to `Paths.Main` only if worktree lookup returns no file
- [x] Unit test: file exists only in worktree (not in main) → verify content is returned

### Task 1.7: Full File Injection on Retry
> Links to: REQ-007

**Acceptance Criteria:**
- [x] Retry instruction includes full file contents under `### Current File Contents ###`
- [x] Each file wrapped in fenced code block with its path
- [x] Applied to `code_backend.go`, `code_frontend.go`, `fix.go`
- [x] Integration test: verify retry prompt contains file contents

## P1 — High

### Task 2.1: Bypass Stale Cache on Retry
> Links to: REQ-008

**Acceptance Criteria:**
- [x] `builder.go` detects retry context and skips `cachedData.SemanticSnippets`
- [x] Calls `ctxEngine.RetrieveContext` dynamically for retry steps
- [x] Non-retry steps still use cache (verify no performance regression)
- [x] Unit test: verify fresh snippets are returned on retry

### Task 2.2: Scope IndexWorkspace to Active Worktree
> Links to: REQ-009

**Acceptance Criteria:**
- [x] When `AgentPathContext` is set, `IndexWorkspace` scans `PhysicalRoot()` only
- [x] Without `AgentPathContext`, existing `code/repos` scan behavior is preserved
- [x] Unit test: verify indexed files are scoped to worktree directory

### Task 2.3: Drop Unresolvable Snippet Paths
> Links to: REQ-010

**Acceptance Criteria:**
- [x] `provider.go:434` uses `continue` instead of `relPath = t.Filepath`
- [x] No absolute host paths appear in any LLM prompt
- [x] Unit test: snippet with out-of-boundary path is dropped, not leaked

## P2 — Medium

### Task 3.1: Sliding Window Error in Retries
> Links to: REQ-011

**Acceptance Criteria:**
- [x] Retry instruction uses `currentInstruction = base + latestError` pattern
- [x] Previous errors are not accumulated
- [x] Applied to `code_backend.go`, `code_frontend.go`, `fix.go`
- [x] Unit test: verify instruction size stable across 3 retries

### Task 3.2: Hunk Line Count Validation
> Links to: REQ-012, REQ-M01

**Acceptance Criteria:**
- [x] `ValidateHunkCounts` function added to `validator.go`
- [x] Called within `ValidateUnifiedDiff` pipeline
- [x] Catches mismatch between declared and actual line counts
- [x] Unit test: `@@ -1,3 +1,19 @@` with 20 lines → fatal validation error

### Task 3.3: Auto-Switch to Search/Replace on Retry
> Links to: REQ-013

**Acceptance Criteria:**
- [x] After 2 unified diff failures, retry instruction requests Search/Replace format
- [x] Instruction includes the `<<<<<<< SEARCH ... >>>>>>> REPLACE` format example
- [x] `SearchReplaceApplier` processes the response correctly
- [x] Integration test: verify format switch occurs on attempt 3

### Task 3.4: RAG Boost for Error Files
> Links to: REQ-014

**Acceptance Criteria:**
- [x] Error file paths are prepended to search query on retry
- [x] `maxSnippets` raised from 4 to 8 for retry attempts
- [x] Broken files mentioned in compiler errors appear in semantic context
- [x] Unit test: verify boosted query returns error file snippets

## P3 — Low

(none)

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
