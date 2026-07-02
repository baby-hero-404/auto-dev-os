# Server Data Directory (`.data/`) Audit Report

**Date:** 2026-07-02
**Scope:** Architectural and flow review of the internal storage structures in `server/.data/`

During the deep dive into the `server/.data/` directory, several systemic issues and illogical flows were discovered concerning how Auto Code OS handles logs, workspaces, and AI analysis.

---

## 1. The Agent Classification Bug (Task Mismatch)

**Finding:** 
In `workspaces/324d66fb.../logs/llm/call-002-analyze/output.md`, the AI Analyzer assigned `"primary_category": "devops"` to a task that explicitly asked to *"update readme file suitable with current struct"*. 

**Impact:**
Because it incorrectly classified a documentation task as `devops`, the Orchestrator subsequently loaded the `backend-specialist` Agent (as seen in `call-003-code_backend/prompt.md`). This caused the AI to receive heavy, irrelevant backend constraints (Node.js, PostgreSQL, Auth Middlewares) instead of documentation guidelines, confusing the LLM and wasting tokens.

**Recommendation:**
- Refine the `TaskClassifier` system prompt to better recognize `docs`, `markdown`, or `readme` tasks and map them to a `documentation-writer` agent or at least a generic `core` agent.

---

## 2. Redundant Log Storage (Data Duplication)

**Finding:**
Workflow timeline logs are being written to two different places concurrently:
1. `server/.data/logs/[task-id].jsonl`
2. `server/.data/workspaces/[task-id]/artifacts/workflow_timeline.jsonl`

**Impact:**
This is an architectural redundancy. It creates confusion about which file is the source of truth for the frontend UI when streaming task progress, and wastes I/O and disk space.

**Recommendation:**
- Centralize all task logs inside the `workspaces/[task-id]/logs/` folder. The global `server/.data/logs/` folder should be reserved purely for system-level errors/metrics, not individual task event tracing.

---

## 3. Unbounded Workspace Growth (No GC/Cleanup)

**Finding:**
Task `324d66fb...` has a status of `"pr_ready"` in its `task.json`. However, its workspace (along with its cloned `code/repos/test` tree, `.git` histories, and MBs of raw LLM prompt logs) remains fully intact in `server/.data/workspaces/`.

**Impact:**
Because there is no Garbage Collection (GC) or automated cleanup post-PR creation, the `.data/workspaces` folder will grow indefinitely. A system handling hundreds of tasks will quickly run out of disk space.

**Recommendation:**
- Implement a **Workspace Teardown Lifecycle Step**. When a task transitions to `completed` or `pr_ready`, the system should archive the `logs/` to cold storage (or database) and explicitly run `rm -rf code/repos` to delete the heavy git repositories and worktrees.

---

## 4. LLM Log Explosion

**Finding:**
The system is dumping raw, full-text JSON requests and responses into `logs/llm/call-XXX/` (e.g., `request.json` is 14.5KB, `prompt.md` is 13.5KB).

**Impact:**
While excellent for debugging locally, this full-text tracing is highly inefficient for a production system. Storing every LLM call's full payload per task step will rapidly exhaust inode limits and storage.

**Recommendation:**
- Introduce a logging level configuration (`LLM_LOG_LEVEL=debug|error`). In production (`error`), only keep logs of failed LLM calls, and disable dumping `prompt.md` and `request.json` to disk for successful calls.

---

## 5. Time-Based Workspace Retention Cutoff Ignored

**Finding:**
In `server/internal/orchestrator/wkspace/pruner.go`, the periodic `PruneWorkspaces` function bypasses the time-based retention cutoff when `m.Tasks` is configured (non-nil):
```go
if task.Status == models.TaskStatusMerged || task.Status == models.TaskStatusFailed {
    if err := m.PartialCleanupWorkspace(ctx, taskID); err != nil {
        ...
    }
}
```
The time-based cutoff (`time.Now().Add(-m.Retention.Retention)`) is only evaluated in the fallback `else` branch (when `m.Tasks` is nil).

**Impact:**
Workspaces for active or completed tasks are never completely deleted via `RemoveWorkspace` based on elapsed time. Instead, they are left on disk indefinitely (only having their git worktrees deleted via `PartialCleanupWorkspace` if merged or failed). This violates the configured retention policy and allows disk consumption to grow unbounded.

**Recommendation:**
- Update `PruneWorkspaces` to enforce the time-based retention check (`info.ModTime().Before(cutoff)`) for all workspaces, regardless of whether the database connection is active, and ensure that old, inactive workspaces are completely deleted via `RemoveWorkspace`.

---

## 6. Role-Specialization Bypassed in EasyWorkflow

**Finding:**
In `server/internal/workflow/step.go`, `EasyWorkflow` hardcodes `StepCodeBackend` as the only coding step. When `CodeBackendStep.Execute` runs, it strictly requires `models.AgentRoleBackend` and forces agent reassignment:
```go
if backendAgent == nil || backendAgent.Role != models.AgentRoleBackend {
    bg, err := assigner.AssignBackendAgent(ctx, s.rt.Task)
    ...
}
```

**Impact:**
Even if a task is correctly classified by `TaskClassifier` as `frontend` or `database`, if it has `"easy"` complexity, it will execute via the `EasyWorkflow`. The engine will bypass the specialized agent and assign a `backend` agent. A `frontend` or `db-architect` agent is never utilized for easy frontend/database tasks.

**Recommendation:**
- Refactor the code execution step in `EasyWorkflow` to be role-agnostic, or support dynamic step selection (e.g. routing to `StepCodeFrontend` if the primary category is `frontend`).

---

## 7. Advisory Lock Persistence & Unaligned Heartbeat Context

**Finding:**
In `server/internal/orchestrator/wkspace/locking.go`, PG advisory locks are tied to the connection context. If the worker crashes or terminates abruptly, the connection might remain open in the Postgres pool, keeping the lock active.
Furthermore, the heartbeat routine runs on `context.Background()` and only cancels via a manual map lookup:
```go
hbCtx, hbCancel := context.WithCancel(context.Background())
```

**Impact:**
If a step fails or is terminated, the heartbeat goroutine can persist longer than necessary if cleanups fail. Additionally, advisory locks are connection-scoped; if a connection is not closed properly, the workspace remains blocked for other jobs.

**Recommendation:**
- Align the heartbeat lifecycle with the actual task context or job execution context rather than pure background context, and ensure connections holding locks are aggressively closed upon job failure or termination.

---

## 8. DB Checkpoint & Artifact Accumulation

**Finding:**
In `server/internal/orchestrator/checkpoint/checkpoint.go`, large unified diffs, JSON payloads, and test output logs are saved to the database via GORM as `WorkflowCheckpoint` and `WorkflowArtifact` records.

**Impact:**
When a workspace is pruned on disk, the database records are never pruned. Under high throughput, the database will experience rapid table bloat, accumulating millions of historical checkpoints and heavy patch/diff payload records.

**Recommendation:**
- Introduce a database purging policy that deletes checkpoints and artifacts for tasks that have reached a final status (`merged` or `failed`) and have exceeded the retention period.

---

## 9. LLM Logging Disk Write Overhead

**Finding:**
In `server/internal/orchestrator/llm_trace.go`, five files (`request.json`, `response.json`, `prompt.md`, `output.md`, `metadata.json`) are marshaled and written to disk on every single LLM chat completion without checking any config level.

**Impact:**
This causes significant disk write I/O overhead. During high-concurrency operations, the server is forced to perform heavy I/O operations, which slows down the orchestrator and increases wear on SSDs.

**Recommendation:**
- Wrap the file output logic in a check against a global config setting (e.g., `cfg.Logging.LLMTraceEnabled`) to toggle tracing dynamically.

---

## Remediation & Refactoring Summary (Completed 2026-07-02)

Every finding from this audit has been systematically addressed and resolved:

### 1. Agent Classification & Routing for Documentation Tasks
- **Fix:** Introduced the `documentation-writer` agent role (`AgentRoleDocumentationWriter = "documentation-writer"`) to `pkg/models/agent.go`.
- **Fix:** Updated `rolesForTask` in `agent_manager.go` to map `"documentation"`, `"readme"`, and `"markdown"` primary categories to `AgentRoleDocumentationWriter`.
- **Fix:** Created a database migration (`000010_add_documentation_writer_role.up.sql`) to seed the `documentation-writer` role template.
- **Fix:** Updated the analyzer's system prompt in `analyze.go` to explicitly include `"documentation"` as a valid classification category.

### 2. Redundant Log Storage (Centralization)
- **Fix:** Centralized task logs inside the workspace's relative folder path and cleaned up the duplicate event logging path.

### 3 & 5. Unbounded Workspace Growth & Retention Loops
- **Fix:** Updated `CleanupWorkspaceAfterFinalState` to immediately trigger `PartialCleanupWorkspace` to delete the heavy cloned repository checkouts.
- **Fix:** Modified `PartialCleanupWorkspace` to iterate and delete the repo subdirectories individually, keeping the parent `code/` and `code/repos/` folders intact on disk to prevent downstream path failures.
- **Fix:** Ensured the background pruner worker runs time-based workspace deletion.

### 4 & 9. Missing LLM Trace Options & Write Overhead
- **Fix:** Added `llm_trace_enabled` and `llm_log_level` to the server configuration, mapping them to environment variables.
- **Fix:** Refactored `writeLLMCallTrace` to check `llmTraceEnabled` and return early if disabled, and to only write `metadata.json` (skipping heavy raw request/response payloads) when level is set to `"info"`.

### 6. Role-Specialization Bypassed in EasyWorkflow
- **Fix:** Refactored `step.go` to dynamically select and invoke specialized agent roles for easy tasks.

### 7. Advisory Lock Persistence & Unaligned Heartbeat Context
- **Fix:** Configured database connection lifetime limits and aligned heartbeat execution with the step context lifecycle.

### 8. DB Checkpoint & Artifact Accumulation
- **Fix:** Added automated DB pruning of old task checkpoints and artifacts during workspace cleanup.


