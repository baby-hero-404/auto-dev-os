# Technical Specifications: Task Detail Status UI

## 1. Helper Function: `isEffectivelyFailed`
**File:** `web/src/lib/status/index.ts`
- **Implementation:**
  Create a new helper function `isEffectivelyFailed(task: Task, workflow: Workflow | null)` that returns `true` if:
  - `task.status === "failed"`
  - OR `workflow?.job?.status === "failed"`
  - OR `(workflow?.job?.status === "paused" && workflow?.job?.last_error != null && workflow?.job?.last_error !== "")`
- **Reasoning:** This ensures we rely on the most recent job execution state, which may be more up-to-date than the task's database row status.

## 2. Retry Affordance & Hero Card Logic
**Files:** 
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeader.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailLayout.tsx`
- **Implementation:**
  - Use the new `isEffectivelyFailed(task, workflow)` helper to determine whether to show the failure hero card instead of the standard running/analyzing indicators.
  - In `TaskHeader` and `TaskDetailLayout`, ensure the "Restart Task" or "Retry Step" action is available whenever the job is effectively failed (even if it's technically in a "paused" state due to a permanent error). The retry button should invoke the context's `retry()` method.

## 3. Workflow Sidebar Sync
**File:** `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSidebar.tsx`
- **Implementation:**
  - Check if the job is effectively failed.
  - Extract the current step from `workflow.job.step`.
  - When rendering the sidebar step list, explicitly mark the step matching `workflow.job.step` with a failed visual state (e.g., a red ✕ icon and red text) if the job has failed. This bypasses the reliance on a specific `status: "failed"` checkpoint record, which doesn't get created in all failure scenarios.

## 4. Checkpoints JSON Formatting
**File:** `web/src/app/projects/[id]/tasks/[taskID]/components/CheckpointsPanel.tsx`
- **Implementation:**
  - When rendering a checkpoint artifact (especially `cli_output`), detect if its content is valid JSON.
  - If it is raw JSON, parse it and present it cleanly:
    - Create a summary header showing key extracted fields (e.g., `duration_api_ms`, `is_error`, `session_id`).
    - Render the rest of the JSON using a structured key-value display or a syntax-highlighted code block instead of a raw text dump.
  - Ensure the JSON view is collapsible for better UX.

## 5. CLI Spec Empty State
**File:** `web/src/app/projects/[id]/tasks/[taskID]/components/CLISpecPanel.tsx`
- **Implementation:**
  - Check if the spec data is empty or missing.
  - Replace the current uninformative empty gray box (which shows `0/0`) with a clear, user-friendly empty state message (e.g., "No specification data available for this task.").
