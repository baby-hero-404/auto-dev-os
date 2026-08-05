# Implementation Tasks: Task Detail Status UI

- [x] **1. Create Status Helper**
  - Update `web/src/lib/status/index.ts` to add the `isEffectivelyFailed` helper function.

- [x] **2. Update Task Hero Cards**
  - Modify `TaskHeroCards.tsx` to use `isEffectivelyFailed` for rendering the failed card immediately when a job fails.

- [x] **3. Update Header and Layout for Retry Actions**
  - Modify `TaskHeader.tsx` to show a Retry/Restart action when paused with an error.
  - Modify `TaskDetailLayout.tsx` to handle the paused banner with an actionable retry button when `last_error` is present.

- [x] **4. Sync Task Sidebar with Job State**
  - Update `TaskSidebar.tsx` to visually mark the current step as failed based on the job state rather than just checkpoint existence.

- [x] **5. Improve Checkpoints JSON Formatting**
  - Update `CheckpointsPanel.tsx` to detect, parse, and prettify JSON artifacts, rendering them as structured collapsible cards.

- [x] **6. Add Empty State for CLI Spec**
  - Update `CLISpecPanel.tsx` to display a helpful empty state message when no spec data is available, replacing the `0/0` display.
