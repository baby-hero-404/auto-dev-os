# Implementation Map: UI Status/Step Display Consolidation

**Goal:** Give task/spec status one source of truth, make CLI-vs-classic step
timelines and the spec-review gate correctly flow-aware, remove dead status
values.
**Tech Stack:** Next.js/React, TypeScript, SWR — `web/src/`.

---

## Phase 1: Central status module + type fixes

### Create `web/src/lib/status/` and align types

**Why:**
This is the foundation every other phase builds on — every later phase
deletes a duplicated switch statement and imports from here instead, so the
module must cover every backend enum value up front.

**Depends on:**
- None.

**Files:**
- `web/src/lib/status/index.ts` (new)
- `web/src/lib/types.ts`

**Changes:**
- [x] In `lib/status/index.ts`, define `getTaskStatusBadge(status: TaskStatus)`
      and `getSpecStatusBadge(status: TaskSpecStatus)` covering every value in
      `server/pkg/models/task.go`'s `TaskStatus*`/`TaskSpecStatus*` consts
      (12 + 8 values), each with a distinct label/variant — no default
      fallback for any real value.
- [x] Add `isTerminalStatus(status: TaskStatus): boolean`,
      `isActiveStatus(status: TaskStatus): boolean`, and
      `needsReview(task: Pick<Task, "status" | "spec_status">): boolean` to
      the same module, consolidating the logic currently duplicated across
      `task-utils.ts`'s `isDoneStatus`/`needsReview`/`isActiveTask`,
      `use-project-data.ts`'s separate `isActiveTask`, and
      `task-action.tsx`'s inline condition.
- [x] In `lib/types.ts`: remove `"planning"` from the `TaskStatus` union; add
      a new `TaskSpecStatus` union (8 values) and retype `Task.spec_status`
      to it instead of `string`.

**Verify:**
- [x] `tsc --noEmit` passes with no new type errors from the `spec_status`
      retype (fix any literal-string comparisons that no longer match the
      union).
- [x] Manually confirm every `TaskStatus`/`TaskSpecStatus` value maps to a
      non-default label by unit-testing (or a quick script) that iterates the
      union and calls both badge functions.

---

## Phase 2: Migrate existing status displays to the central module

**Why:**
Removes the 6+ independent copies so future backend status changes only need
one update site — the core problem this spec exists to fix.

**Depends on:**
- Phase 1 (`lib/status/index.ts` must exist).

**Files:**
- `web/src/components/ui/badge.tsx`
- `web/src/lib/utils/task-utils.ts`
- `web/src/app/analytics/utils.ts`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSidebar.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskTitleBlock.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx`
- `web/src/lib/hooks/use-task-workflow.ts`
- `web/src/components/projects/task-action.tsx`
- `web/src/lib/hooks/use-project-data.ts`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailLayout.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeader.tsx`
- `web/src/components/projects/task-pr-review.tsx`

**Changes:**
- [x] `badge.tsx`: `taskStatusBadge()`/`prStatusBadge()` call into
      `getTaskStatusBadge`/`getSpecStatusBadge` instead of their own switch.
- [x] `task-utils.ts`: `workflowStages`, `isActiveTask`, `isFailedTask`,
      `isDoneStatus`, `needsReview()` delegate to the central module's
      helpers (remove the hand-written duplicates).
- [x] `analytics/utils.ts`: status→color map replaced with a call into the
      central module.
- [x] `TaskSidebar.tsx`, `TaskTitleBlock.tsx`, `TaskHeroCards.tsx`: remove
      each file's own literal status→label/color map (`P`, hero booleans);
      use the central module.
- [x] `use-task-workflow.ts`: `isWorkflowTerminal()` drops the `"done"`/
      `"completed"` checks (values the backend never emits); resolve the
      duplicate `formatStepName` — keep one implementation (prefer
      `TaskSidebar.tsx`'s, since it's the one already exercised in
      production; delete the unused export from `use-task-workflow.ts` or
      re-point it to the kept version) and update all call sites.
- [x] `task-action.tsx`: needs-review condition calls the central module's
      `needsReview()` instead of its own inline check.
- [x] `use-project-data.ts`: delete the file-local `isActiveTask()` function
      (which contains ghost status values `"running"`, `"assigned"`,
      `"in_progress"`) and import `isActiveStatus()` from the central module.
- [x] `TaskDetailLayout.tsx`: replace hardcoded `runningStatuses` array
      (contains `"planning"`) with `isActiveStatus()` from the central module.
- [x] `TaskHeader.tsx`: replace inline `task.status !== "pr_ready" &&
      task.status !== "human_review" && ...` and `task.spec_status` comparisons
      with central module helpers (`isTerminalStatus`, `needsReview`).
- [x] `task-pr-review.tsx`: replace inline `task?.status === "pr_ready"`
      check with a central module helper.
- [x] `analytics/utils.ts`: `STATUS_COLORS` must remove ghost keys
      `"assigned"`, `"in_progress"`, `"completed"`, `"planning"` — only the
      12 backend `TaskStatus` values remain.

**Verify:**
- [x] `grep -rn "case \"failed\"\|case \"merged\"\|case \"pr_ready\"" web/src`
      returns only hits inside `web/src/lib/status/index.ts`.
- [x] Load a task detail page for a `failed` task and confirm the badge is
      red/danger everywhere it appears (sidebar, title block, hero card).
- [x] Load a task with `spec_status: "clarification_required"` and confirm
      it renders a distinct label instead of falling back to neutral.
- [x] `grep -rn '"running"\|"assigned"\|"in_progress"' web/src/lib/hooks/use-project-data.ts`
      returns nothing.
- [x] `grep -rn '"planning"' web/src/app/projects/\[id\]/tasks/\[taskID\]/components/TaskDetailLayout.tsx`
      returns nothing.

---

## Phase 3: Canonical CLI step list (fixes missing `cross_review`)

**Why:**
`cross_review` is a real backend step between `cli_implement` and `cli_mr`
but is currently omitted from every frontend CLI-step list, making its
checkpoint invisible/unlabeled on the timeline.

**Depends on:**
- None (independent of Phases 1-2, can run in parallel).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/CheckpointsPanel.tsx`

**Changes:**
- [x] Define one exported constant (e.g. in `TaskDetailContext.tsx` or a new
      small shared file) listing the CLI flow's step ids in order:
      `["cli_analyze", "cli_spec", "cli_implement", "cross_review",
      "cli_mr"]`.
- [x] `TaskDetailContext.tsx`'s `workflowSteps` memo force-includes from this
      constant instead of its inline array literal.
- [x] `CheckpointsPanel.tsx`'s `STEP_LABELS` adds a `cross_review` entry
      (label: "Cross Review") sourced from/kept in sync with the same
      constant.

**Verify:**
- [x] For a task with a `cross_review` checkpoint, `TaskSidebar.tsx`'s
      workflow-progress stepper shows a "Cross Review" step between
      "CLI Implement" and "CLI Mr".
- [x] `CheckpointsPanel.tsx` shows "Cross Review" (not raw `cross_review`
      snake_case) for the same checkpoint.

---

## Phase 4: Unify flow detection between step timeline and log console

**Why:**
`isCliFlow` (checkpoint-based, in `TaskDetailContext.tsx`) and
`log-console.tsx`'s independent regex-based `cli_` prefix check on raw log
text can disagree for the same task, causing one part of the UI to render
CLI-style while another renders classic-style.

**Depends on:**
- Phase 3 (touches the same context file).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx`
- `web/src/components/dashboard/log-console.tsx`

**Changes:**
- [x] Ensure `isCliFlow` is passed down to (or otherwise accessible by)
      `log-console.tsx` — either as a prop from its parent or via the
      existing task-detail context, whichever the component's current
      mounting point makes simpler.
- [x] `log-console.tsx`'s `groupLogs()` uses the passed-in `isCliFlow` to
      decide flatten-vs-group instead of regexing step names out of log
      message text.

**Verify:**
- [x] For a CLI-flow task, log console flattens CLI step groups as before,
      now driven by the shared flag.
- [x] For a classic-flow task, log console still groups steps normally.

---

## Phase 5: Unify the spec-review gate

**Why:**
Three separate components (`TaskHeroCards.tsx`'s `heroSpec` block,
`CLISpecReviewControls.tsx`, `ReviewActionBar.tsx`) each implement their own
Approve/Request-Changes affordance, and `SpecPanel` + `CLISpecPanel` mount
unconditionally together — a CLI-flow task can show classic placeholder spec
text alongside its real CLI-authored spec, and two button sets can be live
for the same gate at once.

**Depends on:**
- Phase 1 (uses the central module for status checks), Phase 2 (removes the
  status-map duplication this component would otherwise re-introduce).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/CLISpecReviewControls.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/ReviewActionBar.tsx`
- New: `web/src/app/projects/[id]/tasks/[taskID]/components/SpecReviewGate.tsx`

**Changes:**
- [x] Create `SpecReviewGate.tsx`: takes `isCliFlow`, task/spec data, and the
      `approveSpec`/`requestSpecChanges` context actions as props; renders
      `CLISpecPanel` + CLI controls when `isCliFlow`, else `SpecPanel` +
      classic controls.
- [x] `TaskHeroCards.tsx`'s `heroSpec` block renders `<SpecReviewGate />`
      instead of its inline buttons and the unconditional `SpecPanel` +
      `CLISpecPanel` pair.
- [x] Move `CLISpecReviewControls.tsx`'s button logic into `SpecReviewGate`'s
      CLI branch; delete `CLISpecReviewControls.tsx` once nothing else
      imports it.
      **Note:** Code audit confirms `CLISpecReviewControls.tsx` is already
      orphan dead code (zero importers) — it can be deleted outright rather
      than migrated. Still replicate its Approve/RequestChanges UX in
      `SpecReviewGate`'s CLI branch for completeness.
- [x] Remove `ReviewActionBar.tsx`'s spec-review branch (keep its
      human-review/PR-approval branch intact if that part is unrelated to
      this consolidation — confirm via its own call sites before removing).

**Verify:**
- [x] For a task with `status: "spec_review"` and `isCliFlow: true`, only
      `CLISpecPanel` renders, with exactly one Approve/Request-Changes
      control pair.
- [x] For a task with `status: "spec_review"` and `isCliFlow: false`, only
      `SpecPanel` renders, with exactly one Approve/Request-Changes control
      pair.
- [x] `grep -rn "heroSpec\|CLISpecReviewControls" web/src` shows no remaining
      references to the removed inline implementation.

---

## Phase 6: Final sweep

**Why:**
Confirm no duplicated status logic or dead status values remain after the
above phases land.

**Depends on:**
- Phases 1-5.

**Files:**
- Whole `web/src/` tree (search only, no new files).

**Changes:**
- [x] None (verification-only phase).

**Verify:**
- [x] `grep -rn "\"planning\"" web/src` returns no `TaskStatus`-related hits.
- [x] `grep -rn "\"done\"\|\"completed\"" web/src/lib/hooks/use-task-workflow.ts`
      returns nothing.
- [x] `grep -rn "\"running\"\|\"assigned\"\|\"in_progress\"" web/src` returns
      no task-status-related hits (exclude unrelated uses like SWR status
      or job status).
- [x] `grep -rn "CLISpecReviewControls" web/src` returns nothing (file
      deleted).
- [x] `tsc --noEmit` and existing frontend test suite both pass.
