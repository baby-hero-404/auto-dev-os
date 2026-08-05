# Proposal: Enhance Task Detail UI by Status

## Problem

The task detail page has several UX issues that break user trust and make debugging difficult:

1. **Stale status indicator:** When a workflow job fails (e.g. `cli_analyze` returns a permanent auth error), the sidebar Workflow Progress still shows the step as `IN PROGRESS` with a spinning indicator, and the task status badge stays `analyzing`. The user sees "analyzing" but the task is actually dead. The `heroFailed` card only renders when `task.status === "failed"`, but the backend may not have updated the task status yet — the *job* has failed but the *task* row hasn't transitioned.

2. **Missing retry affordance for non-terminal failures:** The `Restart Task` button only appears when `task.status === "failed"`. If the job is `paused` with a `last_error` (e.g. auth failure, config invalid), the only option is the "Boundary Resolution" banner which doesn't always apply. There is no "Retry This Step" or "Resume from Checkpoint" button.

3. **Raw JSON in Checkpoint panel:** Checkpoint artifacts (especially `cli_output` for `cli_analyze`) dump raw JSON blobs inline. Fields like `is_error`, `duration_api_ms`, `session_id` are unreadable to a non-developer user.

4. **Workflow Progress sidebar doesn't reflect `failed` steps:** The sidebar step list derives `failedHere` from `status === 'failed'` in the checkpoint state, but CLI-flow checkpoints only record `success`/`recorded`/`skipped` — a step that errors out never gets a checkpoint with `status: "failed"`, so the sidebar never shows the red ✕ icon.

5. **Empty OpenSpec section:** The `CLISpecPanel` renders an empty gray box with `0/0` when no spec data exists, instead of an informative empty state.

## Goal

Make the task detail page accurately reflect the real-time state of the workflow, provide clear action buttons for every recoverable state, and present error information in a human-readable format.

## Success

- A user looking at a failed task sees a red Failed state immediately (not a blue "analyzing" spinner).
- A user can retry/resume a task from any error state with a single click.
- Checkpoint data is presented as structured, collapsible key-value cards — not raw JSON.

## Assumptions

- The backend `workflow.job.status` and `workflow.job.last_error` fields are the source of truth for whether a step has failed. The task-level `status` field may lag behind.
- The existing `retry()`, `execute()`, and `analyze()` functions in `TaskDetailContext` already call the correct backend endpoints.
- No backend API changes are needed — this is purely a frontend rendering fix.

## Decisions

- **Derive "effectively failed" from job state, not just task status.** If `workflow.job.status === "failed"` OR (`workflow.job.status === "paused"` AND `workflow.job.last_error` contains a permanent error like "invalid configuration"), the UI should show the failed hero card with retry.
- **Add "Retry Step" button to the paused banner.** The existing `BoundaryResolutionControls` only handles clarification boundaries. A generic `retry()` call should be offered when `last_error` is present.
- **Pretty-print JSON in checkpoints.** Detect if artifact content is valid JSON and render it as a collapsible, syntax-highlighted code block with key fields extracted into a summary header.
- **Sync sidebar step status with job state.** If `workflow.job.step === X` and `workflow.job.status === "failed"`, mark step X as failed in the sidebar even without a `status: "failed"` checkpoint.

## Trade-offs

- **Gain:** Immediate visual feedback for all failure modes; actionable retry for stuck tasks.
- **Risk:** Deriving "effectively failed" from job status could show a brief flash of failed state during normal job-to-job transitions. Mitigated by checking `last_error` is non-empty.

## Non-goals

- Redesign the overall task detail page layout or navigation.
- Add new backend API endpoints.
- Change the checkpoint persistence format.

## Out of Scope

- Task list page (project dashboard) status badges — separate task.
- Email/notification on task failure.
- Automatic retry logic in the backend.

## Impact

### Components

- `TaskHeroCards` — add "effectively failed" detection, expand retry button visibility
- `TaskSidebar` — sync step status with job failure state
- `CheckpointsPanel` — pretty-print JSON artifacts, add summary header
- `CLISpecPanel` — add empty state message
- `TaskHeader` — add retry button for paused-with-error state
- `TaskDetailLayout` — ensure paused banner shows retry

### Files

- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSidebar.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/CheckpointsPanel.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/CLISpecPanel.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeader.tsx`
- `web/src/lib/status/index.ts` (add `isEffectivelyFailed` helper)

### Public API

None.

### Migration

None.

### Backward Compatibility

No breaking changes. All changes are additive UI improvements.
