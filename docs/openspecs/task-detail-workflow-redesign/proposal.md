# Proposal: Task Detail Workflow-Oriented Redesign

## Why
A second UX review of the Task Detail dashboard scored it **7.0/10** and reframed the core problem: the page is still a **document viewer**, not a **workflow-execution monitor**. Even after the prior `task-detail-ui-enhancement` pass (hierarchy, spacing, dashboard summary, active-workflow banner — all shipped), a reviewer opening the page cannot answer the six questions that matter in 3–5 seconds:

- What is the AI doing right now?
- What has already been implemented?
- Which unit of work is running?
- What remains?
- Is the workflow waiting for my approval?
- Is anything blocked?

The review traces this to three structural facts still true in `page.tsx` today (verified against the current file):
1. **The primary decision is dead last.** `TaskActions` — which holds *Approve Spec / Request Changes / Start Review* — renders as the **second-to-last** block on the page (`page.tsx:122`), below the logs, spec, and sidebar. A reviewer must scroll the entire page to act.
2. **Phases are shown before real work.** `WorkflowTimeline` (abstract phases: context_load → analyze → plan → code…) renders *above* `ImplementationChecklist` (`page.tsx:71` before `:73`), so the first progress signal a user sees is the least concrete one.
3. **Reference material is always expanded.** `SpecPanel` and `LogConsole` render full-height, always-open (`page.tsx:116`, `:118`), so a long spec document and a raw log stream dominate the fold even though most reviewers read neither on a given visit.

This proposal **supersedes nothing** in `task-detail-ui-enhancement` — that set is complete and its components (`DashboardSummary`, `ActiveWorkflowBanner`, semantic status colors, larger timeline nodes) are the foundation this builds on. This set is the **next evolution**: re-composing those parts, plus the already-built `ImplementationChecklist` / `CurrentImplementationCard`, into a workflow-first layout with a persistent review action bar.

### Feasibility: no backend work
The review's headline ask — show **implementation progress** (per-unit: "Database ✓, Repository ✓, Scheduler 🟠, Retry ⬜") instead of only workflow phases — is already feasible with data the client has. `TaskDetailContext.deriveImplementationItems()` (`TaskDetailContext.tsx:29–85`) builds one `ImplementationItem` per `analysis.execution_unit`, maps it to a `code_{role}_{idx}` step, and derives `done`/`running`/`pending` from `workflow.checkpoints` + the live `job.step`. `implementationItems` and `currentImplementationItem` are already exposed on the context and already consumed by `ImplementationChecklist.tsx` and `CurrentImplementationCard.tsx`. **This is a frontend composition + collapsibility change only — no new endpoint, no schema change.**

## What Changes

### Issue 1: Primary decision is not persistently reachable (Sticky Review Bar — New)
A reviewer must scroll to the bottom (`TaskActions`, `page.tsx:122`) to Approve/Request Changes. Add a **sticky review action bar** pinned above the fold that appears **only when the task is in a state that needs a human decision** — `spec_status ∈ {pending_review, changes_requested}` (Approve Spec / Request Changes) or `status === "pr_ready"` (Start Review). It reuses the exact handlers `TaskActions` already calls (`approveSpec`, `requestSpecChanges`, `startReview` from `useTaskDetail()`) — no second implementation, no new state. When no decision is pending, the bar does not render.

### Issue 2: Phases shown before real implementation work
Reorder so **`ImplementationChecklist` is the primary progress surface** directly under the summary, and **`WorkflowTimeline` is demoted to a secondary "workflow phases" element** below it. The checklist answers "what is actually being built and where does each unit stand"; the phase timeline stays available as the high-level overview, not the headline.

### Issue 3: Specification is always-expanded document
`SpecPanel` renders full-height and always-open (`page.tsx:118`). Make it **collapsed by default**, showing a one-line summary chip row (Scope / Recommendation / Architecture present) with a "View details" expander. The full tabbed document only mounts its body when expanded.

### Issue 4: Execution log dominates the fold
`LogConsole` renders always-open (`page.tsx:116`). Make it **collapsed by default**, showing only the latest event line ("✓ {step} completed" / "▶ running {step}") with a "View full log" expander. The existing grouped, per-step, color-coded log rendering is preserved inside the expanded state — it already *is* the event timeline the review asks for (Issue 6), so no rebuild.

### Issue 5: Everything has equal visual priority
Establish three explicit tiers and enforce them by composition order + elevation:
- **Primary** (always visible, top of page): Sticky Review Bar, `DashboardSummary`, `CurrentImplementationCard`/`ActiveWorkflowBanner`, `ImplementationChecklist`.
- **Secondary** (visible, lighter): `WorkflowTimeline` (phases), `PRPanel`.
- **Tertiary** (collapsed by default): `SpecPanel`, `LogConsole`, `WorkflowSidebar` (Agent Activity, Checkpoints), full `TaskActions` control panel.

### Issue 6: Header project description occupies the first screen
The task description block in `TaskHeader` should be **collapsed by default** ("Show project description" expander), so the header's first paint is title + status strip + inline controls, not a paragraph of prose read once.

### Issue 8: ActiveWorkflowBanner duplicates CurrentImplementationCard
`ActiveWorkflowBanner` ("AI is currently working on {step} → Next: {nextStep}") renders the same running-state information as `CurrentImplementationCard` (running unit + elapsed timer + live file action). The step-level "current + next" is already visible from `WorkflowTimeline` nodes. **Remove `ActiveWorkflowBanner`** — `CurrentImplementationCard` is the strictly richer surface for the same question ("what is the AI doing right now?").

### Issue 9: WorkflowSidebar is redundant with existing surfaces
`WorkflowSidebar` contains three sub-sections, each duplicated elsewhere:
- **Agent Activity** (agent name, current step, attempts, last error, live tool action) — the live tool action duplicates `CurrentImplementationCard.currentAction`. Static metadata (agent, attempts, error) is low-frequency info that can move into `TaskHeader`'s status strip.
- **WorkflowProgress** (completion bar + checkpoints/attempts/files counts) — `DashboardSummary` already shows completion %. Merge the 3 stat boxes into `DashboardSummary` as secondary metrics.
- **Checkpoints list** — `WorkflowTimeline` already visualizes step completion. A raw timestamp list is developer-debug-level data that belongs inside the collapsed `LogConsole`.
**Remove `WorkflowSidebar`** and redistribute its unique data.

### Issue 10: TaskActions is a near-empty shell after the sticky bar
With the Sticky Review Bar owning Approve/Request Changes/Start Review, `TaskActions` retains only Analyze/Execute/Resume + Delete — just 2-3 buttons. This does not justify a standalone card at the page bottom. **Absorb the remaining controls into `TaskHeader`** alongside the existing Pause/Resume/Cancel strip, and keep Delete as a small icon button in the header's overflow/secondary area.

### Issue 7 (already shipped — documented for completeness)
The following the review asks for are **already built** in staged components and only need to be correctly *placed* by this set's composition:
- Task-oriented summary metrics (Tasks Completed / Remaining / Current Task) — `DashboardSummary.tsx` already switches to these when `implementationItems` is non-empty.
- "What the AI is doing right now" with live file/action + elapsed timer — `CurrentImplementationCard.tsx`.
- Live current-step banner + next step — `ActiveWorkflowBanner.tsx`.
- 4-state semantic status colors (emerald/sky/slate/rose) — `getSemanticStatusColor` / `getTaskSemanticStatus` in `TaskDetailContext.tsx`.

## Capabilities

### New Capabilities
- `sticky-review-bar`: persistent, above-the-fold Approve / Request Changes / Start Review bar, shown only while a human decision is pending.
- `workflow-first-composition`: page section order and 2-tier elevation driven by execution priority rather than document order.
- `collapsible-reference-surfaces`: Spec Panel, Log Console, and header description default-collapsed with lightweight summaries.

### Modified Capabilities
- Page composition (`page.tsx`): reordered to primary → secondary → tertiary; reduced from 12 rendered components to 9.
- `ImplementationChecklist`: promoted to the primary progress view (above `WorkflowTimeline`).
- `WorkflowTimeline`: demoted to a secondary "workflow phases" element.
- `TaskHeader`: project description collapsed by default; absorbs Analyze/Execute/Resume/Delete controls from `TaskActions`.
- `DashboardSummary`: absorbs the `WorkflowProgress` bar (checkpoints/attempts/files counts) from `WorkflowSidebar`.

### Removed Capabilities
- `ActiveWorkflowBanner`: removed — its running-state info (current step + next step) is fully covered by `CurrentImplementationCard` + `WorkflowTimeline`.
- `WorkflowSidebar`: removed — Agent Activity live action → `CurrentImplementationCard`; static metadata → `TaskHeader` status strip; `WorkflowProgress` bar → `DashboardSummary`; Checkpoints list → `WorkflowTimeline` (visual) + collapsed `LogConsole` (raw timestamps).
- `TaskActions` (standalone component): removed — review CTAs → `ReviewActionBar`; remaining controls → `TaskHeader` inline controls.

## Impact

| Area | Files Affected | Change |
|------|----------------|--------|
| Page composition | `web/src/app/projects/[id]/tasks/[taskID]/page.tsx` | Reorder + remove 3 component mounts |
| Sticky Review Bar (new) | `web/src/app/projects/[id]/tasks/[taskID]/components/ReviewActionBar.tsx` | Create |
| Implementation Checklist | `web/src/app/projects/[id]/tasks/[taskID]/components/ImplementationChecklist.tsx` | Reposition only |
| Workflow Timeline | `web/src/app/projects/[id]/tasks/[taskID]/components/WorkflowTimeline.tsx` | Demote styling |
| Spec Panel | `web/src/app/projects/[id]/tasks/[taskID]/components/SpecPanel.tsx` | Collapse-by-default |
| Log Console | `web/src/components/dashboard/log-console.tsx` | Collapse-by-default |
| Task Header | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeader.tsx` | Collapse description; absorb Analyze/Execute/Resume/Delete controls |
| Dashboard Summary | `web/src/app/projects/[id]/tasks/[taskID]/components/DashboardSummary.tsx` | Absorb WorkflowProgress stats |
| ActiveWorkflowBanner (remove) | `web/src/app/projects/[id]/tasks/[taskID]/components/ActiveWorkflowBanner.tsx` | Delete file |
| WorkflowSidebar (remove) | `web/src/app/projects/[id]/tasks/[taskID]/components/WorkflowSidebar.tsx` | Delete file |
| TaskActions (remove) | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskActions.tsx` | Delete file; move `WorkflowProgress` export to `DashboardSummary` |
| E2E | `web/e2e/task-detail.spec.ts`, `web/e2e/fixtures/api-mocks.ts` | Update |

## Open Questions
- **ETA / "2 minutes remaining"**: No forecasting model exists. `CurrentImplementationCard` ships an **elapsed** timer; ETA remains out of scope until a duration model exists.
- **Hierarchical checklist grouping (Backend / Frontend / Deployment / Testing)**: Deferred — the default keeps the status grouping (In Progress → Pending → Completed) already built.
- **Sticky bar on mobile**: top-pinned for consistency; open to bottom-bar variant if usability testing prefers it.
