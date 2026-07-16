# Design: Task Detail Workflow-Oriented Redesign

## Design Philosophy
Shift the page's mental model from **"read the document"** to **"watch the run."** The reference is the execution UI of tools like GitHub Actions, Vercel deploys, and Cursor Agent: current status and the primary action are pinned and always reachable; concrete per-unit progress is the headline; abstract phases and reference material (specs, raw logs) recede behind one click. No new design tokens — this reuses the elevation-via-shadow + existing CSS-variable system established by `task-detail-ui-enhancement` (`--stroke`, `--card`, `--surface`, `--brand-primary`, `--success`, `--warning`, `--danger`, `--info`) and the `getSemanticStatusColor` helper already in `TaskDetailContext.tsx`.

## Target Page Composition (`page.tsx`)

```
TaskHeader                    ── PRIMARY  (title + status strip + inline controls + Analyze/Execute/Resume/Delete; description collapsed)
ReviewActionBar               ── PRIMARY  (NEW, sticky; conditional on pending decision)
DashboardSummary              ── PRIMARY  (task-oriented metrics + CurrentImplementationCard + workflow progress stats)
ImplementationChecklist       ── PRIMARY  (moved ABOVE the timeline)
WorkflowTimeline              ── SECONDARY (phases; lighter elevation)
PRPanel                       ── SECONDARY
error / failed / paused banners ── conditional (unchanged blocks)
SpecPanel                     ── TERTIARY  (collapsed by default)
LogConsole                    ── TERTIARY  (collapsed by default)
RequestChangesModal           ── (unchanged)
```

**Removed from composition** (vs. current `page.tsx`):
- `ActiveWorkflowBanner` — deleted (REQ-009). `CurrentImplementationCard` + `WorkflowTimeline` cover its role.
- `WorkflowSidebar` — deleted (REQ-010). Data redistributed to `DashboardSummary` and `TaskHeader`.
- `TaskActions` — deleted (REQ-008). Review CTAs → `ReviewActionBar`; remaining controls → `TaskHeader`.

Diff vs. current `page.tsx`:
- **Add** `<ReviewActionBar />` immediately after `<TaskHeader />`.
- **Move** `<ImplementationChecklist />` from below `<WorkflowTimeline />` (line 73) to above it (before line 71).
- **Remove** `<ActiveWorkflowBanner />`, `<WorkflowSidebar />`, `<TaskActions />`.
- **Wrap** `<SpecPanel />` and `<LogConsole />` in collapse-by-default containers.

The root stack keeps `max-w-7xl space-y-8`.

## Component Design

### `ReviewActionBar.tsx` (new)
A thin client component consuming `useTaskDetail()`. Pure derivation, no local state beyond what the handlers need.

```tsx
const { task, approveSpec, requestSpecChanges, startReview, submittingPR, clarificationQuestions } = useTaskDetail();

const specReview = task && (task.spec_status === "pending_review" || task.spec_status === "changes_requested");
const prReady    = task && task.status === "pr_ready";
if (!specReview && !prReady) return null;   // REQ-001 negative case
```

Layout: `sticky top-0 z-30` bar, `bg-card/80 backdrop-blur border-b border-stroke shadow-sm`, full-bleed within `max-w-7xl`. Left: a "Waiting for your review" label with the semantic **waiting** dot (`getSemanticStatusColor` slate/amber). Right: the actions.
- Spec-review → `Request Changes` (outline/amber) + `Approve Spec` (filled amber, `disabled={clarificationQuestions.length > 0}` with the same tooltip as `TaskActions.tsx:133`).
- `pr_ready` → `Start Review` (filled `bg-brand-primary`, `disabled={submittingPR}`).

Handlers are the **same references** `TaskActions` uses — this is a second render site, not a second source of truth (mirrors how the prior set duplicated Pause/Resume into the header). No new context wiring.

Sticky mechanics: because the page scrolls inside `<main>` (not a nested scroll container), `sticky top-0` on a top-level child works. `z-30` keeps it above cards (`shadow-*`) but below any modal/overlay. On mobile the bar stays top-pinned for consistency (see Open Questions for the bottom-bar alternative).

### `page.tsx` reorder
Only JSX order + one new element; no logic changes. `ImplementationChecklist` already returns `null` when `implementationItems` is empty (`ImplementationChecklist.tsx:9`), so moving it above the timeline is safe for the pre-analysis state — the timeline simply becomes the first visible progress surface then (REQ-003 fallback).

### `ImplementationChecklist.tsx` (built — reposition + confirm primacy)
No structural change required; it already renders In Progress / Pending / Completed groups with sky-pulse running items, emerald strikethrough completed items, file counts, and scroll-to-log on click. This set's work is (a) placing it above `WorkflowTimeline` in `page.tsx`, and (b) confirming its elevation reads as *primary* relative to the demoted timeline (keep its `shadow-lg`; ensure the timeline's is lighter).

### `WorkflowTimeline.tsx` (demote)
Keep all node/connector/pulse behavior from the prior set. Reduce its visual weight relative to the checklist: lighter card elevation (`shadow-sm` rather than `shadow-lg`), and a section label that frames it as "Workflow Phases" (the abstract pipeline) so it doesn't compete with the checklist for "this is the progress" attention. No size regression on the 52px nodes.

### `SpecPanel.tsx` (collapse-by-default)
Wrap the existing panel in a collapse container defaulting to **closed**. Collapsed view = a single row: the panel title + presence chips (Scope / Recommendation / Architecture / Risks) derived from what `analysisData` already contains, plus a `ChevronDown` "View details" affordance — reuse the `isExpandedOpen` toggle pattern already in this file. Expanded view = the current full tabbed body verbatim. The inner Summary-tab collapsibles (Risks / Execution Boundaries / Expanded Boundaries) are untouched and keep working once the outer panel is open. Defer body mount until first expand to keep first paint cheap.

### `log-console.tsx` (collapse-by-default)
Add an outer collapse defaulting to **closed**. Collapsed view = the single latest event line, formatted from the last log/group ("✓ {step} completed", "▶ running {step}", "✗ {step} failed") with the matching semantic color; plus a "View full log" affordance. Expanded view = the existing grouped, per-step, color-coded rendering unchanged. Because the stream stays wired at the page level via `useTaskWorkflow` (the reader loop is not gated by this component's open state), the collapsed summary keeps updating live while running (REQ-006) and no stream state is dropped when closed. Keep the log DOM mounted (hidden via CSS when collapsed) rather than unmounting, so the `log-group-{stepId}` scroll anchors used by `ImplementationChecklist` click-through still resolve.

> **Scroll-anchor resolution (decided)**: `ImplementationChecklist.scrollToLog()` calls `getElementById('log-group-{stepId}')`. Because the collapsed LogConsole keeps its DOM mounted-but-hidden (CSS `hidden` attribute or `max-height: 0; overflow: hidden`), anchors always resolve. However, `scrollIntoView` requires the element to be **painted and visible** — a hidden element will not scroll. The solution is:
>
> 1. **Lift state**: `isLogExpanded` lives in `page.tsx` (or `TaskDetailContext`), not inside `LogConsole`.
> 2. **Pass callback**: `page.tsx` passes `expandAndScrollToLog(stepId: string)` down to `ImplementationChecklist`.
> 3. **Timing**: the callback sets `isLogExpanded = true`, then uses `requestAnimationFrame(() => getElementById('log-group-{stepId}')?.scrollIntoView({ behavior: 'smooth', block: 'start' }))` to wait one paint frame before scrolling — avoids the `setTimeout` anti-pattern while guaranteeing the DOM is visible.
> 4. **LogConsole** receives `isExpanded` + `onToggle` as props instead of owning the toggle internally.

### `TaskHeader.tsx` (collapse description + absorb controls)
Wrap the description prose in a toggle defaulting to **collapsed** ("Show project description"). The status strip (current step / progress % / assigned agent) and inline Pause/Resume/Cancel from the prior set stay outside the toggle, always visible.

**Absorbed from `TaskActions`**: Add Analyze/Execute/Resume buttons to the header's inline controls strip (same row as Pause/Resume/Cancel). These render contextually based on `task.status` and `isExecutionReady`. Delete Task becomes a small de-emphasized icon button (trash icon) in the header's secondary controls area. All handlers are the same references from `useTaskDetail()`.

**Absorbed from `WorkflowSidebar`**: The status strip already shows agent name and current step. Add attempts count and last error as small info chips in the strip (only when non-zero/non-null), keeping the header compact for the normal case.

### `DashboardSummary.tsx` (absorb WorkflowProgress)
The existing `DashboardSummary` already shows task-oriented metrics (Current Task / Tasks Completed / Remaining) or workflow-step metrics as a fallback. **Absorb the `WorkflowProgress` bar** from the removed `WorkflowSidebar`:
- Add a slim completion progress bar below the metric cards (same `bg-brand-primary` bar style).
- Add a secondary row of 3 compact stat chips: checkpoints count, attempts count, files count — mirroring the grid that `WorkflowProgress` rendered.
- The `WorkflowProgress` function currently lives as a named export in `TaskActions.tsx`. Move it (or inline its logic) into `DashboardSummary.tsx` before deleting `TaskActions.tsx`.

## Visual Hierarchy Rules (Issue 5 / REQ-002)

| Tier | Elevation | Members |
|------|-----------|---------|
| Primary | `shadow-lg`, full `bg-card`, semantic accent when active | ReviewActionBar, DashboardSummary (incl. WorkflowProgress), CurrentImplementationCard, ImplementationChecklist |
| Secondary | `shadow-sm`, `bg-card` | WorkflowTimeline, PRPanel |
| Tertiary | collapsed; `shadow-sm`/flat when open | SpecPanel, LogConsole |

Semantic status colors stay as the 4-state overlay (`getSemanticStatusColor`): emerald = done, sky = running, slate = waiting/pending, rose = blocked/failed — applied to the review-bar waiting dot, checklist item states, summary status dot, and timeline rings. The per-step `taskStatusBadge` palette is untouched.

## Data Sources (all already on the context — no backend change)

| Surface | Source |
|---------|--------|
| Pending-decision detection | `task.spec_status`, `task.status`, `clarificationQuestions` |
| Implementation items | `implementationItems`, `currentImplementationItem` (`deriveImplementationItems`) |
| Task-oriented summary | `implementationItems` counts, `workflowCompletion`, `workflowStatusCounts` |
| Current action / elapsed | `logs` (`parseLiveAction`), `workflow.checkpoints[step].created_at` |
| Latest log event (collapsed) | `logs` / grouped log state already in `log-console.tsx` |
| Semantic colors | `getSemanticStatusColor`, `getTaskSemanticStatus` |

## Cross-Component State Coordination

The collapse-by-default surfaces (SpecPanel, LogConsole) introduce a new coordination need: other components must be able to **programmatically expand** them. Rather than adding a new context provider, keep the expanded states as simple `useState` hooks in `page.tsx` and thread them as props:

```tsx
// page.tsx
const [isLogExpanded, setLogExpanded] = useState(false);
const [isSpecExpanded, setSpecExpanded] = useState(false);

const expandAndScrollToLog = useCallback((stepId: string) => {
  setLogExpanded(true);
  requestAnimationFrame(() => {
    document.getElementById(`log-group-${stepId}`)?.scrollIntoView({
      behavior: 'smooth',
      block: 'start',
    });
  });
}, []);
```

This keeps the state co-located with the page layout (where the ordering decisions live) and avoids over-engineering a context for two booleans. If future surfaces also need programmatic expand (e.g., a "View spec" link from elsewhere), the pattern scales by adding another `useState` + callback pair.

**Why not context?** These are page-local UI states with exactly two consumers each (the collapsible surface + the trigger). Context would add indirection without benefit. If a third or fourth consumer emerges, promote to context at that point.

## Testing Strategy
Extend `web/e2e/task-detail.spec.ts` (do not fork a new spec — one Task Detail spec file). New fixture in `api-mocks.ts`: a workflow whose `analysis.execution_units` (≥3 units) plus `checkpoints` produce one `done`, one `running`, and one `pending` implementation item, and whose `task.spec_status`/`status` can be toggled per test to exercise the sticky bar's states. Assertions per REQ-M03: sticky bar visibility × 3 states + handler firing; checklist-above-timeline DOM order; Spec/Log default-collapsed → expand; header description default-collapsed; `ActiveWorkflowBanner`, `WorkflowSidebar`, and `TaskActions` do NOT appear in the DOM. Playwright runs via `npx playwright test` (no `test` script in `package.json`).

## Design Decisions Q&A
- **Why a sticky bar instead of just moving `TaskActions` up?** The review's Issue 10 is specifically about the action being reachable *without scrolling at all* — a non-sticky top panel scrolls away once the reviewer reads the spec/logs, defeating the purpose. Sticky keeps the decision one glance away throughout the review.
- **Why keep `WorkflowTimeline` at all?** Phases (context_load → analyze → plan → code → merge → pr) are the only signal before analysis produces execution units, and they're the high-level "where in the pipeline" view. Demote, don't delete.
- **Why collapse rather than remove Spec/Logs?** They're essential *on demand* (spec for approval detail, logs for debugging) but noise on the fold for the common "is it progressing / does it need me" visit. Collapse serves both.
- **Why remove ActiveWorkflowBanner?** It shows "AI is working on {step} → Next: {nextStep}" — but `CurrentImplementationCard` already shows the running unit + elapsed + live file action (strictly richer), and `WorkflowTimeline` highlights the active node. The banner adds a third surface for the same question.
- **Why remove WorkflowSidebar?** Its 3 sub-sections each duplicate data already on other surfaces: live tool action → `CurrentImplementationCard`, progress bar → `DashboardSummary`, checkpoints list → `WorkflowTimeline`. Removing it cuts ~220 lines and one render tree with no information loss.
- **Why absorb TaskActions into TaskHeader instead of keeping it as a slimmed component?** After the sticky bar takes review CTAs, only 2-3 contextual buttons remain (Analyze/Execute/Resume + Delete). This doesn't justify a standalone card; inlining into the header's control strip keeps actions co-located with the task title and status.
- **Grouping the checklist by role/category?** Deferred (Open Question). Current status-grouping (In Progress → Pending → Completed) already answers "what's running / what's left / what's done"; role grouping (Backend/Frontend/…) is a later refinement, not required for this set.
