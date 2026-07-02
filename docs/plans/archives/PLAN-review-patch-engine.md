# Phase 2: Patch Engine Review (Feature 5.12)

**Feature Spec:** `docs/features/5.12-patch-engine-abstraction.md`  
**Priority:** 🔴 Critical — Code application and self-healing logic  
**Status:** ✅ Review Complete — 2026-07-02

---

## Scope

Verify that the Patch Engine correctly implements:
- `PatchApplier` interface with pluggable strategies
- Unified Diff strategy via `git apply`
- Search & Replace strategy (Aider-style block parsing)
- `PatchValidator` with all 5 validation checks
- In-step self-healing retry (2-3 attempts)
- Integration with coding steps (`code_backend`, `code_frontend`, `fix`)

---

## Files Reviewed

### A. Patch Engine Core (`server/internal/orchestrator/patch/`)

| File | Size | Reviewed |
|:-----|:-----|:---------|
| `types.go` | 0.4KB | ✅ |
| `engine.go` | 2.8KB | ✅ |
| `applier.go` | 12.2KB | ✅ |
| `search_replace.go` | 4.2KB | ✅ |
| `validator.go` | 3.5KB | ✅ |
| `helpers.go` | 6.8KB | ✅ |
| `applier_test.go` | 5.8KB | ✅ |
| `search_replace_test.go` | 1.4KB | ✅ |
| `validator_test.go` | 2.7KB | ✅ |

### B. `engine.go` — Interface

- [x] `PatchEngine` interface has `Validate(patchData, basePath) []ValidationError` ✅
- [x] `PatchEngine` interface has `Apply(ctx, task, agent, stepID, patchData, worktreeSuffix) error` ✅
- [x] Engine selects strategy via `NewEngine(preferredStrategy)` — `"search_replace"` or legacy git ✅
- [x] Engine is pluggable — new strategies implement `PatchEngine` interface ✅

### C. `validator.go` — 5 Validation Checks

- [x] **Repository/File Existence:** `os.ReadFile` + `os.IsNotExist` check ✅
- [x] **Syntax Check:** Hunk header regex validation for Unified Diff ✅
- [x] **Metadata Validation:** Hunk start line vs file line count comparison ✅
- [x] **Uniqueness Check:** `strings.Count == 1` enforcement for S&R blocks ✅
- [x] **File Existence:** Empty SEARCH on non-existent file = create operation (allowed) ✅

### D. `search_replace.go` — Parser

- [x] State machine: `StateNormal → StateSearch → StateReplace → StateNormal` ✅
- [x] Parser handles `File:` metadata prefix (and `file:` lowercase) ✅
- [x] Parser handles backtick wrapping in filepath ✅
- [x] Parser produces `[]EditBlock` with Filepath, Search, Replace ✅
- [x] Parser handles multiple blocks in a single patch string ✅
- [ ] Edge case: empty SEARCH block (file creation) — works but no explicit test
- [ ] Edge case: malformed markers — no guard, could silently skip

### E. `applier.go` — Application

- [x] Unified Diff path: calls runner's `ApplyPatch` (wraps `git apply`) ✅
- [x] S&R path: `strings.Replace(content, search, replace, 1)` — count=1 for safety ✅
- [x] S&R path: `os.MkdirAll` + `os.WriteFile` for new file creation ✅
- [x] Newline normalization: `\r\n` → `\n` before compare and replace ✅
- [x] Groups blocks by file to apply multiple edits atomically ✅

### F. Self-Healing Integration

- [x] `steps/code_backend.go`: Uses `repoutil.ApplyPatch` with retry via checkpoint wrapper ✅
- [x] `steps/code_frontend.go`: Same pattern ✅
- [x] `steps/fix.go`: Same pattern ✅
- [x] On exhausted retries → step returns error → DAG handles routing ✅

### G. Dead Code Found & Fixed

| Item | Status | Action |
|:-----|:-------|:-------|
| Developer TODO comments in multi-repo else-branch | ✅ Fixed | Replaced 8 lines with 2-line design note |

### H. Test Coverage Gaps (Backlog)

| Area | Current | Recommended |
|:-----|:--------|:------------|
| `search_replace_test.go` | 1.4KB (basic) | Add: multi-block, empty SEARCH, empty REPLACE, malformed markers, `File:` prefix |
| Self-healing integration test | None | Add: mock LLM → bad patch → retry → success scenario |
