# Specs: Task Detail Workflow-Oriented Redesign

> Status icons reflect the **current codebase**, verified against the staged components: `✅` shipped · `⚠️` partially present / needs rework · `❌` not started.

## Added Requirements

### REQ-001: Sticky Review Action Bar
> ✅ Status: Shipped

**Scenario: a decision is pending**
- WHEN `task.spec_status ∈ {"pending_review", "changes_requested"}` OR `task.status === "pr_ready"`
- THEN a review action bar MUST be rendered pinned (`position: sticky`) so it stays visible while the user scrolls the page body
- AND for a spec-review state it MUST expose exactly **Approve Spec** and **Request Changes**, wired to the `approveSpec` and `requestSpecChanges` handlers from `useTaskDetail()`
- AND for `pr_ready` it MUST expose **Start Review**, wired to `startReview`
- AND **Approve Spec** MUST be disabled when `clarificationQuestions.length > 0`, matching the existing guard in `TaskActions.tsx`.

**Scenario: no decision pending**
- WHEN the task is running, paused-without-decision, `todo`, `failed`, `merged`, or otherwise has no human decision to make
- THEN the review action bar MUST NOT render (not an empty bar, not a disabled bar).

### REQ-002: Workflow-First Section Order & Tiering
> ✅ Status: Shipped

**Scenario:**
- WHEN a user opens the Task Detail page
- THEN sections MUST render in this order: `TaskHeader` (incl. Analyze/Execute/Resume/Delete controls) → Sticky Review Bar (conditional) → `DashboardSummary` (incl. `CurrentImplementationCard` + workflow progress stats) → `ImplementationChecklist` → `WorkflowTimeline` → `PRPanel` → error/failed/paused banners → collapsed `SpecPanel` → collapsed `LogConsole` → `RequestChangesModal`
- AND `ImplementationChecklist` MUST appear **above** `WorkflowTimeline` in the DOM
- AND `ActiveWorkflowBanner`, `WorkflowSidebar`, and standalone `TaskActions` MUST NOT be rendered as separate components.

### REQ-003: Implementation Checklist Is the Primary Progress View
> ✅ Status: Shipped

**Scenario:**
- WHEN `implementationItems` is non-empty
- THEN `ImplementationChecklist` MUST render as the primary progress surface, above `WorkflowTimeline`
- AND it MUST group items by state (In Progress → Pending → Completed) with the running item(s) shown first and visually emphasized (sky pulse), completed items de-emphasized (strikethrough/emerald)
- AND each item MUST show its name, optional description, and affected-file count, and clicking it MUST expand the LogConsole and scroll to the matching `log-group-{stepId}` anchor
- AND WHEN `implementationItems` is empty (e.g. before analysis), the checklist MUST render nothing and `WorkflowTimeline` remains the visible progress surface.

### REQ-004: Workflow Timeline Demoted to Secondary "Phases"
> ✅ Status: Shipped

**Scenario:**
- WHEN both `ImplementationChecklist` (non-empty) and `WorkflowTimeline` render
- THEN `WorkflowTimeline` MUST read as secondary: lighter elevation than the checklist and positioned below it, labeled as workflow *phases* rather than the headline progress
- AND all existing timeline behavior (running-node pulse/glow, status-colored connectors, 52px nodes from the prior set) MUST be preserved.

### REQ-005: Spec Panel Collapsed by Default
> ✅ Status: Shipped

**Scenario:**
- WHEN the Task Detail page first renders
- THEN `SpecPanel` MUST be collapsed, showing a compact summary row (e.g. presence chips for Scope / Recommendation / Architecture) and a "View details" expander
- AND the full tabbed spec body (Summary/Proposal/Specs/Design/Tasks) MUST only be shown after the user expands it
- AND the existing per-section collapsibles inside the Summary tab (Risks, Execution Boundaries, Expanded Boundaries) MUST continue to work once expanded.

### REQ-006: Log Console Collapsed by Default
> ✅ Status: Shipped

**Scenario:**
- WHEN the Task Detail page first renders
- THEN `LogConsole` MUST be collapsed, showing only the latest event line and a "View full log" expander
- AND expanding it MUST reveal the existing grouped, per-step, color-coded log rendering unchanged
- AND WHEN `isWorkflowRunning` is true, the collapsed summary MUST reflect the live latest event (it MUST NOT freeze on the pre-collapse value).

### REQ-007: Header Project Description Collapsed by Default
> ✅ Status: Shipped

**Scenario:**
- WHEN the Task Detail header renders
- THEN the long project-description prose MUST be collapsed behind a "Show project description" toggle, so the header's first paint is title + status strip + inline controls
- AND the status strip (current step, progress %, assigned agent) and inline Pause/Resume/Cancel controls from the prior set MUST remain always visible, not behind the toggle.

### REQ-008: TaskActions Absorbed into TaskHeader
> ✅ Status: Shipped

**Scenario:**
- WHEN the Sticky Review Bar (REQ-001) owns the pending-decision CTAs (Approve Spec / Request Changes / Start Review)
- THEN those CTAs MUST NOT also appear as standalone buttons elsewhere on the page
- AND `TaskHeader` MUST absorb the remaining non-decision controls: Analyze/Execute/Resume (contextual) + Delete Task (de-emphasized icon button)
- AND the running-state "Workflow Active" placeholder from the old `TaskActions` MUST NOT render — `CurrentImplementationCard` already covers this
- AND the standalone `TaskActions.tsx` component file MUST be deleted.

### REQ-009: Remove ActiveWorkflowBanner
> ✅ Status: Shipped

**Scenario:**
- WHEN the Task Detail page renders
- THEN `ActiveWorkflowBanner` MUST NOT render
- AND its file (`ActiveWorkflowBanner.tsx`) MUST be deleted
- AND the running-state information it displayed (current step name + next step) MUST remain visible via `CurrentImplementationCard` (running unit + live action) and `WorkflowTimeline` (step node highlighting).

### REQ-010: Remove WorkflowSidebar
> ✅ Status: Shipped

**Scenario:**
- WHEN the Task Detail page renders
- THEN `WorkflowSidebar` MUST NOT render
- AND its file (`WorkflowSidebar.tsx`) MUST be deleted
- AND its unique data MUST be redistributed:
  - **WorkflowProgress** bar (completion % + checkpoints/attempts/files counts) → absorbed into `DashboardSummary` as a secondary metrics row (REQ-M01)
  - **Agent Activity** static metadata (agent name, attempts, last error) → absorbed into `TaskHeader`'s status strip
  - **Checkpoints list** → already visualized by `WorkflowTimeline`; raw timestamps available in the collapsed `LogConsole`.

## Modified Requirements

### REQ-M01: Task-Oriented Dashboard Summary (from prior set)
> ✅ Status: Shipped

**Scenario:**
- WHEN `implementationItems` is non-empty
- THEN `DashboardSummary` MUST show task-oriented metrics — Current Task, Tasks Completed (done/total), Remaining Tasks, task-progress % — instead of the raw workflow-step metrics
- AND WHEN `implementationItems` is empty it MUST fall back to the workflow-step metrics (Current Step, Steps Completed, Errors, `workflowCompletion`%)
- AND it MUST include a secondary metrics row showing checkpoints count, attempts count, and files count (absorbed from the removed `WorkflowProgress` component)
- AND the status indicator MUST use the 4-state semantic color (`getSemanticStatusColor(getTaskSemanticStatus(task.status))`).

### REQ-M02: Live "Current Implementation" Surface (from prior set)
> ✅ Status: Shipped

**Scenario:**
- WHEN `workflow.job.status === "running"` AND a `currentImplementationItem` exists
- THEN `CurrentImplementationCard` MUST show the running unit's name, a live elapsed timer from the unit's checkpoint start, and the last parsed tool action (Editing/Running/Reading + target file) derived from `logs`
- AND it MUST render nothing when not running or when there is no current implementation item
- AND it MUST NOT display an ETA/remaining-time field (no forecasting data source exists).

### REQ-M03: E2E Coverage for the Redesigned Composition
> ✅ Status: Shipped

**Scenario:**
- WHEN this redesign ships
- THEN Playwright coverage MUST assert: the Sticky Review Bar appears only for `pending_review`/`changes_requested`/`pr_ready` and fires the correct handler; `ImplementationChecklist` renders above `WorkflowTimeline` when implementation items exist; `SpecPanel` and `LogConsole` start collapsed and expand on interaction; the header description starts collapsed; `ActiveWorkflowBanner`, `WorkflowSidebar`, and standalone `TaskActions` do NOT appear in the DOM
- AND `e2e/fixtures/api-mocks.ts` MUST provide a workflow fixture whose `analysis.execution_units` + `checkpoints` yield a mix of done/running/pending implementation items so the checklist and summary render their task-oriented branches.

## Removed Requirements
- **REQ-R01: ActiveWorkflowBanner** — removed. Running-state info fully covered by `CurrentImplementationCard` + `WorkflowTimeline` nodes.
- **REQ-R02: WorkflowSidebar** — removed. Data redistributed to `DashboardSummary` (progress stats), `TaskHeader` (agent metadata), and `WorkflowTimeline`/`LogConsole` (checkpoints).
- **REQ-R03: TaskActions (standalone)** — removed. Review CTAs → `ReviewActionBar`; remaining controls → `TaskHeader` inline controls.
