# Tasks: Task Detail Workflow-Oriented Redesign

> State tracked via checkboxes. Items already satisfied by staged components are pre-checked with a note; ordered by the review's priority (decision reachability → progress primacy → collapse noise → polish).
> `[x]` = already in the codebase · `[ ]` = to build.

## Phase 0: Foundation already shipped (from `task-detail-ui-enhancement`)
- [x] `DashboardSummary.tsx` renders task-oriented metrics when `implementationItems` is non-empty (Current Task / Tasks Completed / Remaining), falling back to step metrics otherwise. *(REQ-M01)*
- [x] `CurrentImplementationCard.tsx` shows running unit + live elapsed timer + parsed current action, renders null when not running. *(REQ-M02)*
- [x] `ImplementationChecklist.tsx` component exists: In Progress / Pending / Completed groups, file counts, scroll-to-log on click. *(REQ-003 component)*
- [x] `ActiveWorkflowBanner.tsx` shows current step + next step while running. *(prior set — to be removed in Phase 4, REQ-009)*
- [x] `getSemanticStatusColor` / `getTaskSemanticStatus` 4-state color helpers live in `TaskDetailContext.tsx`. *(prior set)*
- [x] `deriveImplementationItems` derives per-unit `done`/`running`/`pending` from `analysis.execution_units` + checkpoints; exposed as `implementationItems` / `currentImplementationItem`. *(data source — no backend work)*

## Phase 1: Sticky Review Action Bar (REQ-001)
- [x] Create `ReviewActionBar.tsx`: consume `useTaskDetail()`; render only when `spec_status ∈ {pending_review, changes_requested}` or `status === "pr_ready"`, else return `null`.
- [x] Spec-review state → `Request Changes` (`requestSpecChanges`) + `Approve Spec` (`approveSpec`, disabled when `clarificationQuestions.length > 0` with the existing tooltip).
- [x] `pr_ready` state → `Start Review` (`startReview`, disabled while `submittingPR`).
- [x] Style as `sticky top-0 z-30` with `bg-card/80 backdrop-blur border-b border-stroke shadow-sm`; left = "Waiting for your review" + semantic waiting dot.
- [x] Mount `<ReviewActionBar />` in `page.tsx` directly after `<TaskHeader />`.

## Phase 2: Workflow-First Composition (REQ-002, REQ-003, REQ-004)
- [x] In `page.tsx`, move `<ImplementationChecklist />` to **above** `<WorkflowTimeline />`.
- [x] Confirm `ImplementationChecklist` reads as primary elevation (`shadow-lg`) vs. a demoted `WorkflowTimeline` (`shadow-sm bg-card/40`).
- [x] Reframe `WorkflowTimeline` as secondary "Workflow Phases" (label + lighter card), preserving node pulse/glow/connectors and 52px nodes.
- [x] Verify the empty-`implementationItems` case: checklist renders nothing (early `null`), timeline is the first visible progress surface.

## Phase 3: Collapse Reference Surfaces (REQ-005, REQ-006, REQ-007)
- [x] `page.tsx`: add `useState` hooks for `isLogExpanded` and `isSpecExpanded` (both default `false`); create `expandAndScrollToLog(stepId)` callback using `requestAnimationFrame` for scroll timing.
- [x] `SpecPanel.tsx`: accept `isExpanded` + `onToggle` props; collapsed = title + presence chips (Scope/Recommendation/Architecture/Risks) + "View details"; body gated behind `isOpen`; inner Summary collapsibles unaffected.
- [x] `log-console.tsx`: accept `isExpanded` + `onToggle` props (externally controlled); collapsed = latest event line (semantic-colored) + "View full log"; body kept mounted-but-`hidden` so `log-group-{stepId}` anchors resolve; latest-event line recomputed each render (live while collapsed).
- [x] `ImplementationChecklist`: accept `expandAndScrollToLog` callback prop; on item click, call it instead of directly invoking `scrollIntoView`.
- [x] `TaskHeader.tsx`: collapse the project-description prose behind "Show project description"; keep status strip + inline Pause/Resume/Cancel always visible.

## Phase 4: Remove Redundant Components & Redistribute Data (REQ-008, REQ-009, REQ-010)
- [x] **Remove `ActiveWorkflowBanner`**: deleted `ActiveWorkflowBanner.tsx`; removed import + mount from `page.tsx`.
- [x] **Remove `WorkflowSidebar`**: deleted `WorkflowSidebar.tsx`; removed import + mount from `page.tsx`.
- [x] **Absorb `WorkflowProgress` into `DashboardSummary`**: progress bar + 3 stat chips (checkpoints/attempts/files) inlined into `DashboardSummary.tsx` as a secondary metrics row (`displayFiles` sourced from context).
- [x] **Absorb remaining `TaskActions` controls into `TaskHeader`**: Analyze/Retry-Analyze + Execute/Retry-Execute buttons added to `TaskHeader`'s controls strip (contextual on `task.status` + `isExecutionReady`); Delete Task = de-emphasized trash icon + `ConfirmDialog`; attempts + last-error surfaced as conditional info chips in the status strip.
- [x] **Remove `TaskActions`**: deleted `TaskActions.tsx`; verified no remaining importers (`WorkflowProgress` migrated first).
- [x] Verified no broken imports (`tsc --noEmit` clean).

## Phase 5: E2E Coverage (REQ-M03)
- [x] Reused the existing `api-mocks.ts` workflow fixture (2 execution_units → 1 running / 1 pending; checkpoints); sticky-bar `pending_review` state supplied via a per-test route override.
- [x] Rewrote `e2e/task-detail.spec.ts`: sticky bar shows for `pending_review` + fires `analysis/approve`, hidden in default running state; checklist-above-"Workflow Phases" DOM order; Spec + Log start collapsed then expand; header description starts collapsed; `AI is currently working on` / `Checkpoints` / `Task Controls` absent.
- [x] `npx playwright test` — all 3 new tests pass; full suite 11/11 green (no regressions).

## Phase 6: Verification
- [x] Reviewer can Approve/Request Changes from the sticky bar without scrolling (E2E-covered).
- [x] Composition renders the workflow-first order; task-oriented summary + checklist above the fold.
- [x] Page renders 9 top-level components (down from 12) with data redistributed, no information loss.
- [x] Lint/typecheck clean (removed 2 pre-existing unused imports in touched files); no new design tokens.
- [x] `specs.md` status icons updated (`❌`→`✅`).
