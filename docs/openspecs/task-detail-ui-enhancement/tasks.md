# Tasks: Task Detail UI/UX Enhancement

> State is tracked using checkboxes. Ordered by the UX review's priority ranking (hierarchy/layout first, responsiveness/awareness last).

## Phase 1: Page Hierarchy & Spacing (REQ-001)
- [x] Reorder sections in `page.tsx`: Header → `DashboardSummary` → `ActiveWorkflowBanner` → status/error/paused banners → `PRPanel` → primary `TaskActions` panel → `WorkflowTimeline` → `SpecPanel`/`LogConsole`/`WorkflowSidebar` grid.
- [x] Change the page's root stack from `space-y-6` to `space-y-8`.
- [x] Audit each component for card-in-card nested borders; replace inner borders with `bg-surface/40` (no border) or shadow-only separation.

## Phase 2: Header & Primary Actions (REQ-002, REQ-005)
- [x] Add a status strip to `TaskHeader.tsx`: current step (`formatStepName`), `workflowCompletion`%, assigned agent (`task.agent_id`, "Unassigned" fallback).
- [x] Move Pause/Resume/Cancel buttons into `TaskHeader.tsx`, wired to the existing `pause`/`execute`/`cancel` handlers from `useTaskDetail()`.
- [x] Remove the now-duplicate Pause/Cancel buttons from `TaskActions.tsx`.
- [x] Split `TaskActions.tsx` into a primary panel (single actionable CTA per task state) and a secondary row (Delete Task only, outline/ghost styling).

## Phase 3: Dashboard Summary & Workflow Awareness (REQ-003, REQ-004)
- [x] Create `DashboardSummary.tsx`: Status, Current Step, Progress %, Steps Done/Total, Error Count, Elapsed Time — sourced from existing `TaskDetailContext` values, no new API calls.
- [x] Create `ActiveWorkflowBanner.tsx`: renders only when `workflow.job.status === "running"`, shows current step name/description and next step name.
- [x] Render both new components in `page.tsx` per the Phase 1 order.

## Phase 4: Task Flow Emphasis (REQ-006)
- [x] Increase `WorkflowTimeline.tsx` node circle size from `size-11` to `size-[52px]` and icon sizes from 13–14px to 16px.
- [x] Add `font-bold` label and a thicker ring (`border-[3px]`) to the currently running node only.
- [x] Verify existing pulse animation, glow, and connector rendering are unaffected by the size change.

## Phase 5: Sidebar Reordering & Collapsibility (REQ-007)
- [x] Reorder `WorkflowSidebar.tsx`: Workflow Progress → Agent Activity → Checkpoints.
- [x] Add a collapse toggle to Checkpoints, default collapsed, showing a "Show N checkpoints" affordance (reuse the `isExpandedOpen` pattern from `SpecPanel.tsx`).

## Phase 6: Spec Panel Collapsible Sections (REQ-008)
- [x] Wrap the Risks Assessment/Risks block in `SpecPanel.tsx` with a collapsible toggle, default expanded when `risk_domains.length > 0`.
- [x] Wrap the Execution Boundaries block with the same toggle pattern, default collapsed.

## Phase 7: Status Color Semantics (REQ-009)
- [x] Define the 4-state semantic color mapping (emerald/sky/slate/rose) as a small shared helper (e.g. `getSemanticStatusColor(status)`), used by `WorkflowTimeline` node rings and `DashboardSummary`/`ActiveWorkflowBanner` status indicators.
- [x] Verify `log-console.tsx`'s existing group status icon colors already match this mapping; adjust only if they diverge.
- [x] Leave `badge.tsx`'s per-step palette (`taskStatusBadge`, etc.) unchanged.

## Phase 8: Responsive Accordion (REQ-M01)
- [x] Build a small shared accordion wrapper (single `openSection` state) for `WorkflowSidebar.tsx`'s Agent Activity and Checkpoints, active below the `md` breakpoint.
- [x] Confirm Workflow Progress stays always-visible and outside the accordion at all breakpoints.
- [x] Confirm the existing `xl:grid-cols-[1fr_380px]` collapse behavior is unchanged.

## Phase 9: E2E Coverage (REQ-010)
- [x] Add route mocks to `e2e/fixtures/api-mocks.ts` for the single-task workflow endpoint (task + checkpoints fixture) and `/tasks/:id/logs/stream` (canned chunked/SSE body).
- [x] Create `e2e/task-detail.spec.ts`: verify header Pause/Resume/Cancel wiring, primary CTA matches mocked spec/task status, Checkpoints default-collapsed, Dashboard Summary and Active Workflow Banner conditional rendering.
- [x] Run `npx playwright test` locally and confirm the new spec passes alongside the existing two.

## Phase 10: Cleanup (Removed Requirements)
- [x] Confirm no `.glass-card` utility or sliding-tab-indicator work was started under the prior spec revision; remove if present.
- [x] Update this task list's checkboxes as each phase ships; do not mark the change complete until Phases 1–8 are done.

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
