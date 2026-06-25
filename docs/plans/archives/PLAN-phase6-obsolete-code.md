# PLAN: Phase 1 — Obsolete & Legacy Code Cleanup (YAGNI)

This sub-plan addresses the removal and consolidation of dead/legacy workflow engine code that was left behind after the workflow steps were refactored into modular step-runner packages.

---

## 1. Scope of Work

The following components inside `server/internal/workflow/` are identified as legacy placeholders that are no longer referenced by the orchestrator engine or any handlers:

| File / Component | Status in Codebase | Proposed Action |
|------------------|--------------------|-----------------|
| `server/internal/workflow/steps.go` | Contains unused functions (`NewAnalyzeStep`, `NewCodeStep`, etc.) and `DefaultWorkflowDefinition`. | **Delete/Prune file contents**. Keep only the package declaration or delete the file completely if no exports are needed. |
| `server/internal/workflow/graph.go` | References legacy comments about `steps.go`. Contains `ValidateDAG` cycle checker. | **Prune comments** referring to `steps.go`. Wire `ValidateDAG` into `validateDefinition` in `engine.go` to guarantee zero-cycle safety at startup/runtime. |

---

## 2. Step-by-Step Execution Plan

### Step 1: Restore and Verify Green Baseline (DONE)
*   Rename `server/internal/orchestrator/step_test.go` to `server/internal/orchestrator/step_testing.go` (if not already done) to solve the production build exclusion bug.
*   Verify the build and run `go test ./...` from the `server` root directory to guarantee we start from a green baseline. Do not make workflow code deletions while the build or tests are red.
*   Confirm that `steps.go` has no active callers in the project via grep search.

### Step 2: Safe Deletion of `steps.go` (DONE)
*   Delete `/home/ubuntu/my_projects/auto_code_os/server/internal/workflow/steps.go`.
*   Verify that `server/` compiles cleanly and no other files references functions inside `steps.go`.

### Step 3: Wire DAG Cycle Checker into Engine Startup Validation (DONE)
*   Modify `validateDefinition(def Definition)` in `server/internal/workflow/engine.go`.
*   Ensure `ValidateDAG(def)` is invoked **after** the existing checks (checking for empty IDs, duplicates, and unknown dependencies) to prevent incomplete graph validation issues in `ValidateDAG`.
*   Verify that cycle detection failures abort engine execution and return the appropriate validation error.

### Step 4: Run local test verification (DONE)
*   Execute `go test ./...` from the backend `server/` root to ensure all package compilations and unit tests pass cleanly.

