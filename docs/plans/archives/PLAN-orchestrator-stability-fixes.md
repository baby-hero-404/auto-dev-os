# PLAN: Orchestrator Stability Fixes

> 4 bugs identified in orchestration engine. Grouped by severity.

**Status:** Complete  
**Created:** 2026-07-03  
**Affects:** `patch/helpers.go`, `patch/applier.go`, `orchestrator.go`, `steps/services.go`, `steps/plan.go`

---

## Progress Summary

| Bug | Status | Notes |
|-----|--------|-------|
| Bug 1 — New file bypass | ✅ Done | `IsUnderAffectedDir` now gates new files and logs warning |
| Bug 2 — Checkpoint pruning | ✅ Done | `ClearCheckpointsForRepair` clears coding + downstream steps |
| Bug 3 — Dead code in skipFE | ✅ Done | Unreachable branch removed |
| Bug 4 — Duplicate subtask mark | ✅ Done | Duplicate/ambiguous block guard added |

### Completed this session (prerequisites):
- ✅ `DeleteCheckpoints` now uses `step = ? OR step LIKE ?_%` pattern matching
- ✅ `updateTaskAnalysis` mutex helper for thread-safe `Task.Analysis` updates
- ✅ `code_backend.go` and `code_frontend.go` migrated to `updateTaskAnalysis`
- ✅ `shouldSkipFrontend` reordered: subtask check (step 2) before category check (step 3)
- ✅ `isFrontendFile` consolidated into `services.go` (single source of truth)
- ✅ New-file patch allowance now requires repo-local ancestry under an affected directory
- ✅ Dynamic checkpoint pruning now clears `code_backend` and `code_frontend` for all repairs
- ✅ Duplicate subtask block replacement now skips ambiguous matches

---

## Bug 1 — Security: New files bypass `affected_files` guard

**Files:** [helpers.go](file:///home/ubuntu/my_projects/auto_code_os/server/internal/orchestrator/patch/helpers.go), [applier.go:75-81](file:///home/ubuntu/my_projects/auto_code_os/server/internal/orchestrator/patch/applier.go#L75-L81)

**Resolved:** New files are only allowed when they are repo-local, safe, and under a directory already represented in `affected_files`.

- [x] Add `IsUnderAffectedDir()` helper in `patch/helpers.go`
- [x] Update enforcement block to require directory ancestry check
- [x] Test: `TestRunner_ApplyPatch_RejectsOutsideAffectedFiles`
- [x] Test: `TestRunner_ApplyPatch_AllowsNewFileUnderAffectedDir`
- [x] Test: `TestIsUnderAffectedDir_AllowsWildcardParentDirectory`

---

## Bug 2 — Resume: `ClearCheckpointsForRepair` doesn't prune dynamic substeps

**File:** [orchestrator.go:224-234](file:///home/ubuntu/my_projects/auto_code_os/server/internal/orchestrator/orchestrator.go#L224-L234)

**Resolved:** `ClearCheckpointsForRepair` now always clears `code_backend`, `code_frontend`, `review`, `fix`, `test`, and `pr`.

**Prerequisite (done):** `DeleteCheckpoints` now uses `step = ? OR step LIKE ?_%` pattern matching, so passing `code_backend` will also delete `code_backend_0`, `code_backend_1`, etc.

- [x] Update `ClearCheckpointsForRepair` in `orchestrator.go`
- [x] Test: `TestOrchestrator_ClearCheckpointsForRepair_PrunesDynamicSubsteps`
- [x] Test: `TestOrchestrator_ClearCheckpointsForRepair_EasyClearsCodingSteps`

---

## Bug 3 — Dead code: `shouldSkipFrontend` unreachable step 4

**File:** [plan.go:150-155](file:///home/ubuntu/my_projects/auto_code_os/server/internal/orchestrator/steps/plan.go#L150-L155)

**Resolved:** `shouldSkipFrontend` now short-circuits on frontend file hints and frontend subtasks, then falls back to category-based skipping. The dead trailing branch was removed.
 
**Previously:** Current order after this session's fix:
1. Check affected files → return false if frontend file found
2. Check `subtasks["frontend"]` > 0 → return false (**new**)
3. Category check → return true if backend/db/devops/security/docs
4. Check `subtasks["frontend"] == 0` → return true (**dead code**)

Step 4 is unreachable: if step 2 didn't return false, then `subtasks["frontend"]` must be 0, so step 4 is always true. The final `return false` after step 4 is also unreachable.

- [x] Remove dead step 4 and unreachable `return false` in `plan.go`
- [x] Test: `TestShouldSkipFrontend_UnknownCategory_NoSubtasks_Skips`
- [x] Test: `TestShouldSkipFrontend_BackendCategory_WithSubtasks_KeepsFrontend`

---

## Bug 4 — Correctness: `updateTaskSubtaskMarkdown` duplicate guard

**File:** [services.go:246-261](file:///home/ubuntu/my_projects/auto_code_os/server/internal/orchestrator/steps/services.go#L246-L261)

**Resolved:** `updateTaskSubtaskMarkdown` now requires exactly one block match before replacement, which prevents ambiguous or duplicate section updates.

- [x] Add block-count guard to `updateTaskSubtaskMarkdown` in `services.go`
- [x] Test: `TestUpdateTaskSubtaskMarkdown_DuplicateBlock_SkipsReplace`
- [x] Test: `TestUpdateTaskSubtaskMarkdown_ReplacesExactBlock`

---

## Execution Order

| Priority | Bug | Risk | Effort | Files |
|----------|-----|------|--------|-------|
| ✅ | Bug 2 — Checkpoint pruning | Resolved | `orchestrator.go` |
| ✅ | Bug 1 — New file bypass | Resolved | `patch/helpers.go`, `patch/applier.go` |
| ✅ | Bug 4 — Duplicate subtask mark | Resolved | `services.go` |
| ✅ | Bug 3 — Dead code in skipFE | Resolved | `plan.go` |

> [!IMPORTANT]
> Verified with targeted `go test` over `internal/orchestrator/patch`, `internal/orchestrator/steps`, and `internal/orchestrator`.
