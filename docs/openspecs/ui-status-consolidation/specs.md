# Expected Behavior: UI Status/Step Display Consolidation

## Scenario: Rendering any `Task.status` value
**When:**
- Any component needs a label/color for `task.status` (badge, sidebar, hero
  card, title block, analytics chart, workflow stage bar).

**Then:**
- It calls the central status module's status-badge function; it does not
  contain its own status→label/color switch.
- Every value in the backend `TaskStatus` enum (`todo, context_loading,
  analyzing, spec_review, coding, reviewing, fixing, testing, pr_ready,
  human_review, merged, failed`) resolves to a distinct, non-default label and
  color — in particular `failed` renders as a clearly distinct (red/danger)
  state everywhere, never falling back to the neutral/default style.

## Scenario: Rendering any `Task.spec_status` value
**When:**
- Any component needs a label/color for `task.spec_status`, or needs to decide
  whether a task "needs review" based on spec status.

**Then:**
- It calls the central status module's spec-status function; the function
  covers all 8 backend `TaskSpecStatus` values (`none, draft, pending_review,
  changes_requested, clarification_required, approved, auto_approved,
  ready_with_warnings`) with a distinct label/color — none fall through to a
  default/neutral rendering.
- `task.spec_status` is typed as the new `TaskSpecStatus` union in
  `lib/types.ts`, not `string`.

## Scenario: CLI-flow task step timeline
**When:**
- A task is running the CLI-spec-first flow (`isCliFlow` true) and has a
  `cross_review` checkpoint recorded.

**Then:**
- `cross_review` appears in the step list rendered by `TaskSidebar.tsx`, in
  its correct position between `cli_implement` and `cli_mr`.
- `CheckpointsPanel.tsx` shows a proper label for `cross_review` (not a raw
  snake_case fallback).
- Both components source the CLI step list from the same shared constant —
  there is exactly one place in the codebase enumerating CLI step ids.

## Scenario: Log console CLI-step grouping matches flow detection
**When:**
- `log-console.tsx` decides whether to flatten a step's logs (CLI-style) or
  group them (classic-style).

**Then:**
- It uses the `isCliFlow` value computed in `TaskDetailContext.tsx` (or a
  value derived identically from checkpoint data) — it does not independently
  regex-match step names out of raw log message text to make this decision.

## Scenario: Spec-review gate rendering (either flow)
**When:**
- `task.status === "spec_review"`.

**Then:**
- Exactly one review-gate UI surface is rendered for the task: it shows
  `CLISpecPanel` + CLI-appropriate controls when `isCliFlow` is true, or
  `SpecPanel` + classic controls when `isCliFlow` is false — never both
  panels, and never two independent sets of Approve/Request-Changes buttons
  mounted at once.
- Approving or requesting changes from this single gate calls the same
  `approveSpec`/`requestSpecChanges` context actions regardless of flow type.

## Scenario: Dead status values are unreachable
**When:**
- Searching the frontend codebase for `"planning"` (as a `TaskStatus` value)
  or `"done"`/`"completed"` (as job/workflow-terminal status checks).

**Then:**
- No such branches exist; `TaskStatus` in `lib/types.ts` matches the backend
  `TaskStatus*` constants exactly, and `isWorkflowTerminal()` only checks
  status values the backend can actually produce.

## Scenario: Ghost status values are removed from all helpers
**When:**
- Searching the frontend codebase for `"running"`, `"assigned"`,
  `"in_progress"`, or `"completed"` as task-status string literals.

**Then:**
- No `isActiveTask()`/`runningStatuses` array or `STATUS_COLORS` map contains
  any of these values. `use-project-data.ts`'s duplicate `isActiveTask()` no
  longer exists — it imports from the central module.
- `analytics/utils.ts`'s `STATUS_COLORS` only contains keys matching the
  backend `TaskStatus` enum exactly (12 values).
- `TaskDetailLayout.tsx`'s auto-expand logic uses the central module's
  `isActiveStatus()` instead of a hardcoded `runningStatuses` array.

## Scenario: Inline status/spec_status checks delegate to central module
**When:**
- A component needs to decide behavior based on `task.status` or
  `task.spec_status` (e.g., canResume, showAnalyze, isPrReady).

**Then:**
- It calls a named helper from the central status module (e.g.,
  `isTerminalStatus()`, `needsReview()`, `isActiveStatus()`) rather than
  writing inline `task.status === "pr_ready"` comparisons.
- In particular: `TaskHeader.tsx`'s canResume/showAnalyze logic,
  `task-pr-review.tsx`'s pr_ready check, and `task-action.tsx`'s
  `isPendingSpecReview` all use central module helpers.

## Rules
- No component may define its own `Task.status` → label/color mapping; all
  such logic lives in `web/src/lib/status/`.
- No component may define its own `isActiveTask()` or `runningStatuses` array;
  the central module's `isActiveStatus()` is the single implementation.
- The frontend `TaskStatus` and `TaskSpecStatus` unions must be kept in exact
  1:1 correspondence with `server/pkg/models/task.go`'s `TaskStatus*` and
  `TaskSpecStatus*` constants (same value set, no extras, no omissions).
- Flow detection (`isCliFlow`) has exactly one implementation, computed from
  checkpoint data; every other consumer reads that value rather than
  re-deriving it.
- The spec-review gate has exactly one mounted implementation per task at any
  time — never two panels, never two button sets.
- `CLISpecReviewControls.tsx` is deleted (orphan dead code, zero importers) —
  not migrated.

## Constraints
- No backend API/contract changes — this is a frontend-only consolidation.
- No changes to SWR polling intervals or data-fetching behavior.
- Existing visual design tokens (colors, spacing) are reused for the newly
  added status cases; no new design system introduced.
