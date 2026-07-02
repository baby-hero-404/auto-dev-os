# Phase 1: Workflow Engine Review (Feature 5.7)

**Feature Spec:** `docs/features/5.7-workflow-engine.md`  
**Priority:** 🔴 Critical — Core orchestration pipeline  
**Status:** ✅ Review Complete — 2026-07-02

---

## Scope

Verify that the DAG-based workflow engine correctly implements:
- Step ordering and dependency resolution per complexity level
- State machine transitions (12 states)
- Checkpoint/recovery (resume from last successful step)
- Review-fix cycle limits (`max_review_fix_cycles`)
- Human gate pause/resume mechanics
- Parallel coding ownership (BE/FE branch isolation)
- Error handling and bounded retry

---

## Files Reviewed

### A. Workflow Definition Layer (`server/internal/workflow/`)

| File | Size | Review Focus |
|:-----|:-----|:-------------|
| `engine.go` | 6.7KB | DAG execution loop, step dependency resolution, retry logic |
| `graph.go` | 1.1KB | Step dependency graph construction |
| `schema.go` | 1.3KB | Workflow schema definitions (Easy/Medium/Hard graphs) |
| `state_machine.go` | 1.6KB | Task status transitions — verify all 12 states match spec |
| `step.go` | 4.5KB | Step interface, step state definitions |
| `engine_test.go` | 6.1KB | Test coverage for DAG execution |
| `state_machine_test.go` | 3.9KB | Test coverage for state transitions |

**Checklist:**
- [x] `schema.go` — Not used for step sequences. Step sequences defined in `step.go` via `EasyWorkflow`, `MediumWorkflow`, `HardWorkflow` functions
- [x] `step.go` defines correct step sequences per complexity:
  - Easy: `context_load → analyze → code_backend → test → pr` ✅
  - Medium: 10 steps with BE/FE fan-out, merge, review, fix ✅
  - Hard: Reuses Medium (planned cross-harness expansion) ✅
- [x] `state_machine.go` delegates to `models.ValidTaskTransitions` — all 12 states match spec 5.6 ✅
- [x] `engine.go` — `CompletedSteps` map enables checkpoint resume ✅
- [x] `engine.go` — `Resume()` skips completed steps, handles `ErrPaused` / `ErrWaitingApproval` ✅
- [x] `engine.go` — Parallel execution capped by `MaxParallel` (default 4) ✅

### B. Orchestrator Core (`server/internal/orchestrator/`)

**Checklist:**
- [x] `orchestrator.go` correctly dispatches to workflow engine based on task complexity ✅
- [x] `worker.go` respects workspace locking via `wkspace` package ✅
- [x] `step_registry.go` registers ALL 10 steps referenced in the workflow schemas ✅
- [x] `agent_manager.go` routes model level to Gateway correctly ✅
- [x] Dead code scan: `test_runner.go` and `llm_step.go` are NOT dead — actively used via step_registry ✅

### C. Step Implementations (`server/internal/orchestrator/steps/`)

**Checklist:**
- [x] `context_load.go`: Loads repo structure, conventions, CI config, ARCHITECTURE.md ✅
- [x] `analyze.go`: Generates OpenSpec, outputs complexity + risk_domains ✅
- [x] `analyze.go`: Human gate logic via `policy.ShouldAutoApproveSpec()` — auto-approve Easy+low-risk ✅
- [x] `code_backend.go` / `code_frontend.go`: Uses Patch Engine (repoutil.ApplyPatch) ✅
- [x] `code_backend.go` / `code_frontend.go`: Runs targeted tests after coding ✅
- [x] `merge.go`: Handles parallel branch merging for Medium/Hard tasks ✅
- [x] `review.go`: Implements cross-review (different agent selection) ✅
- [x] `fix.go`: Bounded by checkpoint/workflow retry limits ✅
- [x] `testing.go`: Runs tests in sandbox ✅
- [x] `pr.go`: PR template includes summary, changes, risk assessment, test results ✅

### D. Checkpoint & Recovery (`server/internal/orchestrator/checkpoint/`)

- [x] Checkpoints saved via `WithCheckpointRecovery` wrapper in step_registry ✅
- [x] Recovery correctly identifies last successful step ✅
- [x] Recovery tests exist in `recovery_test.go` ✅

### E. Workspace Management (`server/internal/orchestrator/wkspace/`)

- [x] Workspace structure matches spec layout (specs, context, artifacts, logs, pr) ✅
- [x] Locking mechanism in `locking.go` with TTL-based distributed locks ✅
- [x] Pruner in `pruner.go` preserves artifacts, handles cleanup ✅

### F. Dead Code Found & Fixed

| Item | Status | Action |
|:-----|:-------|:-------|
| `StepCode = "code"` constant | ✅ Fixed | Removed — replaced by `StepCodeBackend` + `StepCodeFrontend` |
| `DescribeStep("code")` entry | ✅ Fixed | Replaced with `"code_backend"` and `"code_frontend"` entries |
| `workspace/` vs `wkspace/` duplication | ✅ Verified | NOT duplicated — different purposes (paths vs lifecycle) |
| `test_runner.go` (0.9KB) | ✅ Verified | NOT dead — used in step_registry (3 references) |
| `llm_step.go` (1.2KB) | ✅ Verified | NOT dead — used in step_registry (5 references) |
