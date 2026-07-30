# Proposal: UI Status/Step Display Consolidation (CLI + API flows)

## Problem
The frontend renders task/spec status through **8+ independently written mappings**
(`badge.tsx`, `task-utils.ts` ×2, `use-project-data.ts`, `analytics/utils.ts`,
`TaskSidebar.tsx`, `TaskTitleBlock.tsx`, `TaskHeroCards.tsx`,
`use-task-workflow.ts`) instead of one shared source. Each was hand-copied from
the backend `TaskStatus`/`TaskSpecStatus` enums (`server/pkg/models/task.go`) at
different times, so they've drifted:

- `taskStatusBadge()` has no `"failed"` case (renders gray instead of red).
- `prStatusBadge()` is missing `clarification_required` and `ready_with_warnings`.
- The frontend `TaskStatus` union includes `"planning"`, which has no backend
  constant — likely dead.
- `isWorkflowTerminal()` checks job-status values `"done"`/`"completed"` that
  don't exist anywhere in the backend or the frontend type.
- `spec_status` is typed as a bare `string` (no union), so every consumer does
  ad hoc string comparisons with no compiler safety.
- `use-project-data.ts`'s own `isActiveTask()` (a **third** copy, separate from
  `task-utils.ts`'s version) includes ghost status values `"running"`,
  `"assigned"`, and `"in_progress"` that have no backend constant — inflating
  active-task counts on the project dashboard.
- `analytics/utils.ts`'s `STATUS_COLORS` map contains additional ghost keys
  `"assigned"`, `"in_progress"`, and `"completed"` beyond the already-noted
  `"planning"`.
- `TaskDetailLayout.tsx` hardcodes `runningStatuses` with `"planning"` to
  control auto-expanding log/checkpoint sections.
- `TaskHeader.tsx` has inline `task.status`/`task.spec_status` comparisons
  for canResume/showAnalyze logic, bypassing any central module.
- `task-pr-review.tsx` inline-checks `task?.status === "pr_ready"` outside
  the central module.

Separately, the CLI-spec-first flow (`cli_analyze → cli_spec → cli_implement →
cross_review → cli_mr`) and the classic flow (`analyze → plan → code → review →
fix → test → pr`) share the same `Task.status` field but are rendered by
partially-overlapping, partially-divergent UI:

- `cross_review` — a real backend step between `cli_implement` and `cli_mr` —
  is omitted from every frontend list of CLI steps (`TaskDetailContext.tsx`'s
  force-included `["cli_analyze","cli_spec","cli_implement","cli_mr"]`,
  `CheckpointsPanel.tsx`'s `STEP_LABELS`), so its checkpoint is invisible or
  unlabeled on the step timeline.
- Flow detection is implemented twice, independently: `isCliFlow` in
  `TaskDetailContext.tsx` (derived from checkpoint step-name prefixes) vs. a
  separate regex-based `cli_` prefix check in `log-console.tsx`'s log grouping.
  These can disagree for the same task.
- Three separate components implement Approve/Request-Changes for the
  spec-review gate (`TaskHeroCards.tsx`'s `heroSpec` block,
  `CLISpecReviewControls.tsx`, `ReviewActionBar.tsx`), and the classic
  `SpecPanel` + CLI `CLISpecPanel` are mounted unconditionally together
  whenever `status === "spec_review"`, regardless of which flow the task is
  actually running — so a CLI-flow task can show placeholder/synthesized spec
  text from the classic panel alongside its real CLI-authored spec.
- `CLISpecReviewControls.tsx` is currently **orphan dead code** — no file in
  `web/src/` imports it (grep confirms only self-references). It can be
  deleted outright rather than migrated.

Net effect: status coloring, step timelines, and review-gate controls behave
inconsistently between CLI-mode and API-mode tasks, and any future backend
status change requires updating 8+ call sites to stay in sync — which is
exactly how the current drift happened.

## Goal
Give task/spec status a single source of truth on the frontend, and make the
step-timeline and spec-review-gate UI correctly flow-aware (CLI vs. classic)
without duplicated detection logic or duplicated approval components.

## Success
A task's status color/label is visually identical everywhere it appears on a
given page load, `failed` and every live `spec_status` value render distinctly
(no silent fallback-to-neutral), `cross_review` is visible on the CLI step
timeline when its checkpoint exists, and there is exactly one Approve/Request-
Changes control surface active per spec-review gate — never two panels or two
button sets for the same task.

## Decisions
- **Central status module** (`web/src/lib/status/`) becomes the only place
  that maps `TaskStatus`/`TaskSpecStatus` → label/color/terminal-ness. All
  existing call sites are migrated to import from it instead of maintaining
  their own switch statement.
- **`spec_status` gets a real union type** in `lib/types.ts`, mirroring
  `TaskSpecStatus*` in `server/pkg/models/task.go` exactly (8 values), instead
  of staying a bare `string`.
- **CLI step list becomes one constant** (includes `cross_review`), consumed by
  both `TaskDetailContext.tsx` and `CheckpointsPanel.tsx`, replacing the two
  independently hardcoded lists.
- **Flow detection stays as the existing `isCliFlow` computation** in
  `TaskDetailContext.tsx` (checkpoint-based, already the more reliable of the
  two signals since it reads structured data, not log text) — `log-console.tsx`
  is changed to consume `isCliFlow` from context instead of re-deriving it from
  log text via regex.
- **One spec-review-gate component**, parameterized by `isCliFlow`, replaces
  `TaskHeroCards`'s inline `heroSpec` buttons, `CLISpecReviewControls.tsx`, and
  the spec-review branch of `ReviewActionBar.tsx`. It renders `CLISpecPanel`
  when `isCliFlow` is true and `SpecPanel` otherwise — never both.
- Dead status values (`"planning"` in the frontend `TaskStatus` union,
  `"done"`/`"completed"` in `isWorkflowTerminal()`, `"running"`/`"assigned"`/
  `"in_progress"` in `use-project-data.ts`, and the matching ghost keys in
  `analytics/utils.ts`'s `STATUS_COLORS`) are removed rather than kept
  "just in case" — grep confirms the backend never emits them.
- `CLISpecReviewControls.tsx` is **deleted outright** (orphan dead code —
  zero importers) rather than migrated into `SpecReviewGate`.

## Trade-offs
- Touches ~15 frontend files in one pass instead of many small independent
  patches — higher review surface for this change, but avoids a partial fix
  that would leave some of the 8+ status maps out of sync with the others.
- Consolidating three approval components into one increases that single
  component's conditional complexity (branches on `isCliFlow`), but removes
  the risk class where two approval surfaces are live for the same task at
  once.
- We keep `isCliFlow`'s existing checkpoint-based detection rather than
  redesigning flow detection from scratch — this is a pragmatic choice to
  avoid scope creep; if checkpoint-based detection itself proves unreliable,
  that is a separate follow-up.

## Out of Scope
- No backend changes. `pkg/models/task.go`'s enums are the source of truth we
  align to, not something we modify here.
- No change to polling/data-fetching behavior (`use-task-workflow.ts`'s SWR
  refresh interval, the separate `task-artifacts` SWR key, or SSE log
  streaming) — the staleness risk from independent SWR keys is noted but not
  addressed in this pass.
- No redesign of the visual design system (colors/spacing) beyond what's
  needed to cover the missing status cases — this is a correctness/consistency
  fix, not a restyle.
- No changes to `TaskSubtasks`/`SupportingAccordion.tsx`'s own `isCliFlow`
  consumption beyond having it read from the now-canonical source.

## Impact
- `web/src/lib/status/` (new) — central status/spec-status mapping module.
- `web/src/lib/types.ts` — `TaskStatus` union (remove `"planning"`),
  new `TaskSpecStatus` union, `Task.spec_status` retyped.
- `web/src/components/ui/badge.tsx` — `taskStatusBadge()`/`prStatusBadge()`
  delegate to the central module.
- `web/src/lib/utils/task-utils.ts` — `workflowStages`, `isActiveTask`,
  `isFailedTask`, `isDoneStatus`, `needsReview()` delegate to the central
  module.
- `web/src/app/analytics/utils.ts` — status→color map delegates to the
  central module.
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx`
  — canonical CLI step-list constant (adds `cross_review`); exposes
  `isCliFlow` for `log-console.tsx` to consume.
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSidebar.tsx`,
  `TaskTitleBlock.tsx`, `TaskHeroCards.tsx` — delegate to central module;
  `TaskHeroCards.tsx`'s `heroSpec` block replaced by the new unified gate
  component.
- `web/src/app/projects/[id]/tasks/[taskID]/components/CheckpointsPanel.tsx`
  — `STEP_LABELS` sourced from the canonical CLI step-list constant.
- `web/src/app/projects/[id]/tasks/[taskID]/components/CLISpecReviewControls.tsx`,
  `ReviewActionBar.tsx` — spec-review-gate logic merged into the new unified
  gate component; both files' spec-review-specific code is removed once
  merged.
- `web/src/components/dashboard/log-console.tsx` — CLI-step-grouping reads
  `isCliFlow` from context instead of its own regex heuristic.
- `web/src/lib/hooks/use-task-workflow.ts` — remove the dead
  `"done"`/`"completed"` branches in `isWorkflowTerminal()`; remove the unused
  duplicate `formatStepName` export once `TaskSidebar.tsx`'s inline version is
  confirmed as the one kept (or vice versa — see tasks.md Phase 2).
- `web/src/components/projects/task-action.tsx` — needs-review condition
  delegates to the central module's `needsReview()`.
- `web/src/lib/hooks/use-project-data.ts` — delete the duplicate
  `isActiveTask()` (contains ghost statuses `"running"`, `"assigned"`,
  `"in_progress"`); import the central module's `isActiveStatus()` instead.
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailLayout.tsx`
  — replace hardcoded `runningStatuses` array (contains `"planning"`) with
  the central module's `isActiveStatus()` helper.
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeader.tsx` —
  inline `task.status`/`task.spec_status` comparisons for canResume/
  showAnalyze delegate to central module helpers.
- `web/src/components/projects/task-pr-review.tsx` — inline `"pr_ready"`
  check delegates to a central module helper.
