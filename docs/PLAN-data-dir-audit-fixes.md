# Plan: Data Directory Audit Fixes

This plan outlines the specific steps required to implement the remaining fixes identified in the `data-dir-audit-report.md` that were missing from the codebase.

## 1. Finding 5: Enforce Time-Based Workspace Retention
**File:** `server/internal/orchestrator/wkspace/pruner.go`
- [ ] Refactor `PruneWorkspaces` to evaluate `info.ModTime().Before(cutoff)` for **all** workspaces.
- [ ] Move the time-based check outside of the `else` block so that even if the database connection (`m.Tasks`) is active, old inactive workspaces will still be completely deleted via `m.RemoveWorkspace(entry.Name())`.

## 2. Finding 6: Role-Agnostic Coding for Easy Tasks
**Files:** `server/internal/workflow/step.go` & `server/internal/orchestrator/steps/code_backend.go`
- [ ] **Approach:** We will implement a generic `StepCode` (or a `CodeDispatcherStep`) that dynamically routes to the appropriate underlying code logic based on `task.Analysis.PrimaryCategory`. Alternatively, we'll relax the role requirement in `CodeBackendStep` if the complexity is easy.
- [ ] Update `EasyWorkflow` in `step.go` to use this more flexible step definition instead of forcing `StepCodeBackend`.

## 3. Finding 7: Fix Heartbeat Context Leak (Advisory Locks)
**File:** `server/internal/orchestrator/wkspace/locking.go`
- [ ] Locate `AcquireWorkspaceLock` where the heartbeat context is created (`hbCtx, hbCancel := context.WithCancel(context.Background())`).
- [ ] Change the base context from `context.Background()` to the incoming execution `ctx` (`context.WithCancel(ctx)`). This ensures the heartbeat goroutine accurately mirrors the job's lifecycle and terminates if the job crashes or is cancelled.

## 4. Finding 8: DB Checkpoint & Artifact Pruning
**Files:** `server/internal/orchestrator/wkspace/pruner.go` and/or `lifecycle.go`
- [ ] Inject `m.Workflows.DeleteCheckpoints()` and an equivalent artifact cleanup call into `PruneWorkspaces` or `RemoveWorkspace`.
- [ ] When an old workspace is permanently deleted from disk (because it exceeds the retention cutoff), ensure its associated heavy DB checkpoint records and artifacts (like patch diffs) are also purged from the database to prevent table bloat.

## Testing & Validation
- [ ] Verify `PruneWorkspaces` correctly deletes an old workspace and its DB records.
- [ ] Validate `EasyWorkflow` correctly assigns and uses a Frontend agent for a frontend-classified easy task.
- [ ] Ensure cancelling a task gracefully terminates the workspace heartbeat goroutine.
