# Design: Task Detail UI/UX Enhancement

## Architecture & Styling Strategy
This revision replaces the prior "glass-card" (blur + nested translucent border) direction with an **elevation-via-shadow system**: hierarchy is communicated by shadow depth, background contrast, and spacing, not by adding more borders. Cards that need to visually recede (secondary sidebar content) get flat `bg-surface/40` with no border; cards that need to stand out (Dashboard Summary, Primary Task Actions) get `shadow-md` and a slightly stronger `bg-card`. Only one border per visual "region" — never a bordered card containing another bordered card, per Issue 10 of the review.

All work stays within existing Tailwind utility classes and the project's existing CSS variables (`--stroke`, `--card`, `--surface`, `--brand-primary`, `--success`, `--warning`, `--danger`, `--info`); no new design tokens are introduced.

## Page Composition (`page.tsx`)

```
TaskHeader                     (status strip + inline quick actions)
DashboardSummary                (new — status/phase/progress/errors/runtime)
ActiveWorkflowBanner            (new — conditional on workflow.job.status === "running")
error / paused / boundary banners (unchanged, existing conditional blocks)
PRPanel                          (unchanged)
TaskActions primary panel        (moved above the fold — see Component Notes)
WorkflowTimeline                 (Task Flow — larger nodes)
grid xl:grid-cols-[1fr_380px]
  ├─ SpecPanel + LogConsole      (main column)
  └─ WorkflowSidebar             (Workflow Progress, Agent Activity, Checkpoints)
RequestChangesModal              (unchanged)
```

`space-y-6` (24px) on the page's root stack becomes `space-y-8` (32px), matching Issue 9 of the review (24–32px between major sections).

## Component Notes

### `TaskHeader.tsx`
Add a status strip row beneath the existing title/badges: current step name (`formatStepName(workflow.job.step, analysisData)`), `workflowCompletion`%, and assigned agent (`task.agent_id`, same "Unassigned" fallback `WorkflowSidebar` already uses). Add inline Pause/Resume/Cancel buttons sourced from the same `pause`/`execute`/`cancel` handlers `TaskActions` already uses via `useTaskDetail()` — no new context wiring, just rendered in two places with the same source of truth. `TaskActions`'s own Pause/Cancel buttons are removed once they're in the header (avoid duplicate controls).

### `DashboardSummary.tsx` (new)
A single-row metric strip, `shadow-md` card, placed directly under the header:

| Metric | Source |
|--------|--------|
| Status | `task.status` via `taskStatusBadge` |
| Current Step | `formatStepName(workflow.job.step, analysisData)` |
| Progress | `workflowCompletion` (%) |
| Steps Done / Total | `workflowStatusCounts.done` / `workflowSteps.length` |
| Error Count | `workflowStatusCounts.failed` |
| Elapsed Time | `now - new Date(workflow.checkpoints[0].created_at)`, formatted with the same `Xm Ys` logic already used in `stepDurations` |

Renders `null` if `workflow` isn't loaded yet (same guard pattern `SpecPanel` uses for `task?.analysis`).

### `ActiveWorkflowBanner.tsx` (new)
Rendered only when `workflow?.job?.status === "running"`. Content: "AI is currently working on **{formatStepName(currentStep)}**" + `getStepDescription(currentStep, analysisData)`, plus "Next: {formatStepName(nextStep)}" where `nextStep` is `workflowSteps[workflowSteps.indexOf(currentStep) + 1]` (omit the "Next" line if it's the last step). No ETA — see proposal.md Open Questions.

### `TaskActions.tsx`
Split the current single `flex flex-wrap` action row into:
1. **Primary panel** — exactly one of: Approve Spec/Request Changes (spec review state), Execute/Retry Execute (ready-to-run state), Resume (paused, not in spec review), Start Review (PR ready). Rendered with the existing filled `bg-brand-primary`/`bg-amber-500` styling, larger (`py-2.5`) than secondary actions.
2. **Secondary row** — Delete Task only (Pause/Cancel move to the header per Issue 2); kept as the existing outline/danger-outline button style, visually smaller and positioned below the primary panel so it reads as "not the main decision here."

The Workflow Progress card (progress bar + checkpoints/attempts/files counters) is unchanged in content, only reordered per Issue 7 (see `WorkflowSidebar.tsx` below).

### `WorkflowSidebar.tsx`
Order becomes: Workflow Progress (from `TaskActions`) → Agent Activity → Checkpoints. Checkpoints gets a collapse toggle (`useState`, default `false`/collapsed) with a "Show N checkpoints" affordance, mirroring the existing `isExpandedOpen` pattern already used for `analysisData.expanded_boundaries` in `SpecPanel.tsx` — same interaction, no new pattern introduced.

### `WorkflowTimeline.tsx`
- Node circle: `size-11` (44px) → `size-[52px]`.
- Icon size passed to `getStepIcon`/inline icons: 13–14px → 16px.
- Active node label: add `font-bold` and bump ring width (`border-2` → `border-[3px]`) only for `isRunning`, so the current step is unambiguous even before reading any label.
- No change to the existing pulse animation, connector rendering, or hover tooltip — those already satisfy the review's "distinct states" ask.

### `SpecPanel.tsx`
Wrap the "Risks Assessment"/"Risks" block and the "Execution Boundaries" block in the same collapsible pattern already implemented for `expanded_boundaries` (`isExpandedOpen` + `ChevronDown`/`ChevronUp`). Default state: **open** if the section has content the user likely needs immediately (Risks open when `risk_domains.length > 0`), **collapsed** otherwise (Execution Boundaries, since it's reference material once the spec is approved).

### `log-console.tsx`
No structural change — the existing grouped, collapsible, color-coded-by-level rendering already is the "event timeline" the review's Issue 6 asks for. Only the semantic color pass from Issue 9 below touches this file (group status icons already use blue/green/red/amber, which already matches the 4-state scheme — verify, don't rebuild).

## Status Color Semantics (Issue 9)

The review asks for a single 4-color status scheme. Collapsing `badge.tsx`'s 12-hue per-step palette to 4 colors would make different workflow steps visually indistinguishable in the header/badges (todo vs. analyzing vs. planning vs. coding would all render identically), which is its own hierarchy regression. Instead, apply a **second, additive** semantic layer used only where the review actually needs "am I looking at something done/running/blocked/waiting" at a glance — timeline node rings, `DashboardSummary` status indicator, `ActiveWorkflowBanner`:

| Semantic state | Color | Applies to |
|---|---|---|
| Completed | `emerald-500` (existing `--success`) | Timeline node ring, summary status dot |
| Running | `sky-500` (existing `--info`) | Timeline node ring, summary status dot, active banner |
| Waiting/Pending | `slate-400` / `--stroke` | Timeline node ring, summary status dot |
| Blocked/Failed | `rose-500` (existing `--danger`) | Timeline node ring, summary status dot |

`badge.tsx`'s per-step palette (`taskStatusBadge`, etc.) is untouched — it continues to answer "which step" while the new overlay answers "is it fine."

## Design Decisions Q&A

**Design system adherence** — none. Reuse existing tokens/utilities only (`--stroke`/`--card`/`--surface`/`--brand-primary`/`--success`/`--warning`/`--danger`/`--info`, `custom-scrollbar`, `Badge`/`taskStatusBadge`, and the `isExpandedOpen` collapsible-toggle pattern already in `SpecPanel.tsx`). No new "glassmorphism" tokens or blur utilities are introduced — see Removed Requirements in specs.md.

**Where controls live relative to status/logs** — Pause/Resume/Cancel are workflow-level controls, not task decisions, so they move out of the sidebar into the header (REQ-002) rather than living alongside status indicators there. The sidebar keeps only the single primary decision CTA (`TaskActions` primary panel) plus status/read-only surfaces (Workflow Progress, Agent Activity, Checkpoints). Logs are not sidebar content at all — they stay in the main `1fr` column next to `SpecPanel`.

**Log streaming placement** — embedded in the main layout (current position, not a drawer). `log-console.tsx` already renders its own internal collapsible step-groups; nesting that inside an outer drawer would be a collapsible-inside-a-collapsible, working against REQ-001's "no card-in-card" rule. The stream itself (`api.streamLogs` → `/tasks/{id}/logs/stream`, fetch+`ReadableStream`, auto-reconnect with backoff, buffered through `useRealtimeLogStore`) is wired at the page level via `useTaskWorkflow`; keeping the consumer always-mounted avoids any risk of a collapsed/closed drawer silently dropping stream state.

## Testing Strategy

No test coverage exists for this route today — verified directly, not assumed:
- **Component tests**: no Jest/Vitest config anywhere under `web/`; zero unit tests for any `tasks/[taskID]` component.
- **E2E**: `web/e2e/` has two Playwright specs (`auth-and-dashboard.spec.ts`, `git-accounts.spec.ts`); neither navigates into `/projects/[id]/tasks/[taskID]`. `web/e2e/fixtures/api-mocks.ts` mocks the task *list* endpoint but has no mock for single-task workflow status/checkpoints or the `/tasks/{id}/logs/stream` endpoint.
- `package.json` has no `test` script; Playwright runs via `npx playwright test`.

This means the enhancement needs **new** E2E coverage, not an update to existing tests. New fixtures required in `api-mocks.ts`: a route for `GET /api/v1/tasks/:id/workflow` (or whatever `api.taskWorkflow` hits) returning a task + checkpoints fixture, and a route/stub for `/tasks/:id/logs/stream` (can return a small canned chunked body — the reader loop only needs valid SSE-shaped lines to exercise the UI, it doesn't need a real long-lived stream in tests). A new `web/e2e/task-detail.spec.ts` should cover: header quick actions firing the right handler, primary CTA visibility per spec/task status, Checkpoints starting collapsed, and the Dashboard Summary/Active Workflow Banner rendering conditionally on `workflow.job.status`.

## Responsive Rules (Issue 10)

- `xl` and above: unchanged `grid-cols-[1fr_380px]` split.
- `md`–`xl`: sidebar already collapses below the main column (existing grid behavior) — no change needed.
- Below `md`: `WorkflowSidebar`'s Agent Activity and Checkpoints render through a shared `<Accordion>`-style wrapper (single `openSection` state, one of `"agent" | "checkpoints" | null`) instead of two independently-scrolling always-open cards, reducing vertical scroll on small screens. Workflow Progress (top-priority per Issue 7) stays always-visible, not part of the accordion.
