# Proposal: Task Detail UI/UX Enhancement

## Why
An external UX review of the Task Detail dashboard scored it **6.8/10**, identifying the root causes as **high information density**, **weak visual hierarchy**, and **low action discoverability** — not a lack of visual polish. Every section (`TaskHeader`, `PRPanel`, `WorkflowTimeline`, `SpecPanel`, `LogConsole`, `WorkflowSidebar`) currently uses the same `rounded-xl border border-stroke bg-card p-5 shadow-sm` treatment, so nothing signals what the user should look at first, what needs a decision, or what to do next.

This revision **supersedes the prior "Glassmorphism" direction** of this spec set. That proposal targeted decorative polish (blur, nested borders, sliding tab indicators) which the review's Issue 10 (Excessive Borders) explicitly argues against — piling translucent bordered layers on top of an already border-heavy layout would make the hierarchy problem worse, not better. Two pieces of that prior work are kept because they already satisfy what the review is asking for and are live in the codebase today:
- `WorkflowTimeline.tsx` already has pulse/glow animations on the running node and status-colored connectors (prior REQ-002).
- `log-console.tsx` already groups raw logs into collapsible, status-colored step groups (prior REQ-004) — this *is* the "event timeline" the review asks for in Issue 6; no rebuild needed, only the color-consistency pass in Issue 9 below.

Everything else in the prior spec (glass-card blur/border utility, sliding tab background, skeleton loading) is dropped or deferred — see "Removed Capabilities".

## What Changes

### Issue 1: Page-Level Visual Hierarchy & Section Order
`page.tsx` renders sections in an order that doesn't match how a user actually needs to scan the page (header → error banners → `PRPanel` → `WorkflowTimeline` → `SpecPanel`/`LogConsole`/`WorkflowSidebar`). Reorder to: **Header → Dashboard Summary → Active Workflow Banner (conditional) → Primary Task Actions → Task Flow → (Spec Panel + Log Console) / Sidebar**. Replace the nested `border border-stroke` card-in-card pattern with shadow + background-contrast separation, and increase inter-section spacing from `space-y-6` (24px) to `space-y-8` (32px) per Issue 9/Issue 10 of the report.

### Issue 2: Header Lacks Operational Context
`TaskHeader.tsx` currently shows only title, status/spec/priority badges, and description — no phase, progress, or quick actions; those are buried in `WorkflowSidebar`/`TaskActions`, off-screen on first view. Add a compact status strip (current phase, progress %, assigned agent) directly to the header, and surface Pause/Resume/Cancel inline there so control doesn't require scrolling to the sidebar. (Branch/owner/runtime are **not** included — see Open Questions, no such data is currently exposed by `TaskDetailContext`.)

### Issue 3: Missing Dashboard Summary (New)
Users must currently read three different components (`TaskActions` progress bar, `WorkflowSidebar` agent card, `WorkflowTimeline` status badges) to answer "where does this task stand?". Add a `DashboardSummary` panel directly under the header showing Status, Current Step, Progress %, Steps Done/Total, Error Count, and Elapsed Time — all derivable from data `TaskDetailContext` already computes (`workflowCompletion`, `workflowStatusCounts`, `stepDurations`, `workflow.checkpoints`), so no backend changes are required.

### Issue 4: Limited Workflow Awareness (New)
There is no way to tell what the agent is doing right now beyond a small "Active: {step}" pill inside `WorkflowTimeline`. Add an `ActiveWorkflowBanner`, shown only while `workflow.job.status === "running"`, stating the current step's human-readable name + description (`getStepDescription`) and the next step in `workflowSteps`. No ETA field — not derivable from existing data (see Open Questions).

### Issue 5: Primary Actions Lack Emphasis
`TaskActions.tsx` renders Approve Spec / Request Changes / Execute / Pause / Cancel / Delete as a single `flex-wrap` row of similarly-weighted buttons — the decision that's actually blocking the task (e.g. Approve Spec) has no more visual weight than Delete. Split into two groups: a **primary panel** with the single actionable decision for the task's current state and a filled primary CTA, and a **secondary row** (Pause/Cancel/Delete) with de-emphasized outline/ghost styling.

### Issue 6: Task Flow Readability
`WorkflowTimeline.tsx` node circles are `size-11` (44px) with 13–14px icons; the running node already pulses/glows, but at this size + icon weight the current step doesn't stand out enough when scanning quickly. Increase circle size to 52px, icon size to 16px, and bump the active node's label to `text-foreground font-bold` with a stronger ring.

### Issue 7: Sidebar Prioritization
`WorkflowSidebar.tsx` stacks `TaskActions` (itself two cards), Agent Activity, and Checkpoints at equal, always-expanded weight — a compact space with too much always-visible detail. Reorder to Primary Actions → Workflow Progress → Agent Activity → Checkpoints, and default Checkpoints to **collapsed** (it's an audit trail, not a decision surface).

### Issue 8: Specification Section Length
The `SpecPanel` Summary tab already has tab-level navigation (Summary/Proposal/Specs/Design/Tasks), but the Summary tab itself is one long scroll of Risks, Affected Files, Execution Boundaries, and (only) Expanded Boundaries is collapsible today. Extend the same collapsible pattern to Risks and Execution Boundaries so a user isn't forced to scroll past sections they don't currently need.

### Issue 9: Inconsistent Status Colors
`badge.tsx`'s `taskStatusBadge` maps each workflow status to one of 12 distinct hues (indigo, purple, cyan, violet, orange, teal, yellow, ...) purely for step identification — there's no consistent "done/running/blocked/waiting" signal a user can scan for across components. Layer a 4-state semantic color (green=done, blue=running, gray=waiting, red=blocked/failed) onto `WorkflowTimeline` node rings and the `DashboardSummary`/`ActiveWorkflowBanner` status indicators, **on top of** (not replacing) the existing per-step badge palette, which stays useful for telling steps apart. See design.md for the reasoning behind not collapsing the whole badge palette to 4 colors.

### Issue 10: Responsive Behavior
Below `md`, `WorkflowSidebar`'s Agent Activity and Checkpoints render as two always-expanded stacked cards, which is expensive on a small screen. Convert them to an accordion (one section open at a time) below the `md` breakpoint.

## Capabilities

### New Capabilities
- `dashboard-summary-panel`: at-a-glance status/phase/progress/error/runtime strip.
- `active-workflow-banner`: live "what is the agent doing right now" banner, shown only while running.

### Modified Capabilities
- Task Detail header: adds phase/progress/agent + inline Pause/Resume/Cancel.
- Task action panel: primary decision vs. secondary/destructive action separation.
- Workflow timeline: larger nodes/icons, stronger current-step emphasis.
- Spec Panel summary: Risks and Execution Boundaries become collapsible.
- Sidebar: reordered, Checkpoints collapsed by default, mobile accordion.
- Status color semantics: 4-state overlay applied to timeline/summary/banner.

### Removed Capabilities
- Nested `border border-stroke` card-in-card pattern (replaced by shadow/spacing separation).
- Prior "Glassmorphism theme" direction (blur + translucent nested borders) and its sliding-tab-indicator requirement — dropped as incompatible with the border-reduction goal above. Timeline animations and log grouping from the same prior spec are **kept** (already implemented, already satisfy the new review).
- Skeleton loading states — out of scope for this revision (not raised by the new review); left for a future, separate pass if still desired.

## Impact

| Area | Files Affected |
|------|-----------------|
| Page composition | `web/src/app/projects/[id]/tasks/[taskID]/page.tsx` |
| Header | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeader.tsx` |
| Dashboard Summary (new) | `web/src/app/projects/[id]/tasks/[taskID]/components/DashboardSummary.tsx` |
| Active Workflow Banner (new) | `web/src/app/projects/[id]/tasks/[taskID]/components/ActiveWorkflowBanner.tsx` |
| Task Actions | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskActions.tsx` |
| Sidebar | `web/src/app/projects/[id]/tasks/[taskID]/components/WorkflowSidebar.tsx` |
| Task Flow | `web/src/app/projects/[id]/tasks/[taskID]/components/WorkflowTimeline.tsx` |
| Spec Panel | `web/src/app/projects/[id]/tasks/[taskID]/components/SpecPanel.tsx` |
| Execution Log | `web/src/components/dashboard/log-console.tsx` (color-token pass only) |
| Status colors | `web/src/components/ui/badge.tsx` |

## Open Questions
- **Branch / Owner / Runtime in header**: the report asks for these, but `TaskDetailContext` exposes no repo branch and no human-readable task owner today (`task.agent_id` is a raw ID, "Unassigned" fallback already exists in `WorkflowSidebar`). Elapsed Time is derivable from `workflow.checkpoints[0].created_at` → now/last checkpoint; Branch/Owner are deferred until that data is available, and are **not** part of this spec's Header requirement (REQ-002).
- **Estimated Remaining time** in the Active Workflow Banner: no duration-estimation model exists (only *past* `stepDurations`, no forecast). The banner ships without an ETA field.
- Whether the per-step badge palette (12 hues) should eventually collapse to the report's literal 4-color scheme, or keep both layers as proposed in Issue 9 — flagged for design review, not decided by this proposal.
