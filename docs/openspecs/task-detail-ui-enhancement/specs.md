# Specs: Task Detail UI/UX Enhancement

## Added Requirements

### REQ-001: Page Section Hierarchy
> ❌ Status: Not Started

**Scenario:**
- WHEN a user opens the Task Detail page
- THEN sections MUST render in this order: Header, Dashboard Summary, Active Workflow Banner (if running), status/error/paused banners, PR Panel, Primary Task Actions, Task Flow, then the Spec Panel/Log Console/Sidebar grid
- AND vertical spacing between top-level sections MUST be 32px (`space-y-8`), not 24px
- AND no card MAY contain a second bordered card nested directly inside it (one border per visual region).

### REQ-002: Header Operational Context
> ❌ Status: Not Started

**Scenario:**
- WHEN a user views the Task Detail header
- THEN it MUST display, in addition to the existing title/description/badges: the current workflow step, progress percentage, and assigned agent
- AND it MUST expose Pause, Resume, and Cancel controls inline, without requiring the user to scroll to the sidebar
- AND these controls MUST use the same `pause`/`execute`/`cancel` handlers already provided by `useTaskDetail()`, not a second implementation.

### REQ-003: Dashboard Summary Panel
> ❌ Status: Not Started

**Scenario:**
- WHEN the workflow data has loaded
- THEN a summary panel MUST render directly below the header showing: Status, Current Step, Progress %, Steps Done/Total, Error Count, and Elapsed Time
- AND all values MUST be derived from data already computed by `TaskDetailContext` (`workflowCompletion`, `workflowStatusCounts`, `stepDurations`, `workflow.checkpoints`) — no new backend endpoint
- AND the panel MUST render nothing (not a skeleton, not an error) while `workflow` is undefined.

### REQ-004: Active Workflow Awareness Banner
> ❌ Status: Not Started

**Scenario:**
- WHEN `workflow.job.status === "running"`
- THEN a banner MUST display the current step's human-readable name and description (via `formatStepName`/`getStepDescription`) and the name of the next step in `workflowSteps`, if any
- AND the banner MUST NOT render when the job is not running (paused, queued, done, failed)
- AND the banner MUST NOT display an estimated-remaining-time field (no data source exists for it; see design.md Open Questions).

### REQ-005: Primary vs. Secondary Task Actions
> ❌ Status: Not Started

**Scenario:**
- WHEN a task has an actionable decision available (spec review, ready-to-execute, paused-and-resumable, or PR-ready)
- THEN exactly one primary action panel MUST render that action with a filled, high-contrast CTA
- AND Pause/Cancel controls MUST move to the header (REQ-002), leaving only Delete Task in a visually de-emphasized secondary row within `TaskActions`
- AND the secondary row MUST use outline/ghost button styling, not filled buttons, so it never competes visually with the primary CTA.

### REQ-006: Task Flow Node Emphasis
> ❌ Status: Not Started

**Scenario:**
- WHEN the Task Flow (`WorkflowTimeline`) renders its nodes
- THEN each node circle MUST be at least 52px with a 16px icon (up from 44px/13px)
- AND the currently running node's label MUST render in `font-bold` with a visibly thicker ring than completed/pending nodes
- AND existing pulse animation, status glow, and connector-line behavior MUST be preserved unchanged.

### REQ-007: Sidebar Prioritization
> ❌ Status: Not Started

**Scenario:**
- WHEN a user views the right sidebar
- THEN sections MUST appear in this order: Workflow Progress, Agent Activity, Checkpoints
- AND the Checkpoints section MUST default to collapsed, with a toggle showing the checkpoint count (e.g. "Show 5 checkpoints")
- AND expanding Checkpoints MUST NOT cause Workflow Progress or Agent Activity to shift position or collapse.

### REQ-008: Specification Collapsible Sections
> ❌ Status: Not Started

**Scenario:**
- WHEN a user views the Spec Panel's Summary tab
- THEN the Risks and Execution Boundaries subsections MUST be independently collapsible, using the same toggle pattern already implemented for JIT Expanded Boundaries
- AND Risks MUST default to expanded when `risk_domains` is non-empty, and collapsed otherwise
- AND Execution Boundaries MUST default to collapsed.

### REQ-009: Status Color Semantics
> ❌ Status: Not Started

**Scenario:**
- WHEN a workflow step, timeline node, or dashboard summary status indicator is displayed
- THEN it MUST use one of exactly four semantic colors based on state — emerald for completed, sky for running, slate for waiting/pending, rose for blocked/failed
- AND this semantic layer MUST be additive to (not a replacement for) the existing per-step badge palette in `badge.tsx`, which continues to distinguish individual workflow steps.

## Modified Requirements

### REQ-M01: Responsive Sidebar Behavior
> ❌ Status: Not Started

**Scenario:**
- WHEN viewing the page below the `md` breakpoint
- THEN Agent Activity and Checkpoints MUST render inside a shared accordion where only one of the two is expanded at a time
- AND Workflow Progress MUST remain always visible, outside the accordion
- AND the existing `xl:grid-cols-[1fr_380px]` → single-column collapse at `xl` and below MUST be unchanged.

### REQ-010: E2E Coverage for Task Detail
> ❌ Status: Not Started

**Scenario:**
- WHEN this enhancement's page-hierarchy, header, and action-panel changes ship
- THEN a new Playwright spec MUST exist covering: header quick actions (Pause/Resume/Cancel) invoking the correct handler, the primary action panel showing exactly the CTA matching the mocked task/spec status, Checkpoints rendering collapsed by default, and the Dashboard Summary / Active Workflow Banner appearing only when `workflow.job.status` matches their respective conditions
- AND `e2e/fixtures/api-mocks.ts` MUST gain route mocks for the single-task workflow endpoint and the `/tasks/:id/logs/stream` endpoint, since neither exists today.

## Removed Requirements

### Requirement: Glassmorphism Theme
**Reason**: The new UX review's Issue 10 (Excessive Borders) explicitly recommends reducing nested-border/translucent-layer usage in favor of shadows and spacing; a backdrop-blur glass-card treatment on top of an already border-heavy layout works against that goal and against the hierarchy fixes in REQ-001.
**Migration**: No code from the prior `.glass-card` utility ships; cards use the elevation-via-shadow system in design.md instead.

### Requirement: Premium Tab Navigation (sliding indicator)
**Reason**: Cosmetic-only change with no bearing on the review's findings (information hierarchy, action discoverability); deprioritized in favor of the structural changes above. The existing static active-tab highlighting in `SpecPanel.tsx` is retained as-is.
**Migration**: None required — no removal of working code, only removal from this spec's scope.

### Requirement: Skeleton Loading States
**Reason**: Not raised by the new UX review; out of scope for this revision to keep the change set focused on hierarchy/discoverability.
**Migration**: Existing `<Loader2 className="animate-spin" />` loading state in `page.tsx` is unchanged. May be revisited in a future, separate proposal.
