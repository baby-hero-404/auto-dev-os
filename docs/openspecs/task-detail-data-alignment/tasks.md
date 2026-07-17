# Tasks: Task Detail Data Alignment

## Fixes & UI Adaptations

### Setup
- [x] Fix the unescaped template literal syntax error in `BoundaryResolutionControls.tsx`.

### TaskHeroCards Adjustments
- [ ] Refactor `heroSpec` to combine multiple Markdown fields from `analysisData` if available (proposal, specs, design, tasks), providing a richer definition-of-ready view.
- [ ] Refactor `heroExec` to properly map `logs`, optionally pinning to bottom or slicing correctly to prevent layout overflow.
- [ ] Ensure empty states for `heroLoad` checkpoints map safely to `workflow.checkpoints`.

### TaskSidebar Adjustments
- [ ] Rewrite the Workflow Stepper in `TaskSidebar.tsx`. Instead of using hardcoded `phaseDefs`, map over `workflowSteps` provided by the context.
- [ ] Use the `latest` status map from `useTaskDetail()` to accurately determine step active/done/failed states and colors, ensuring true data accuracy even if it deviates slightly from the static design mock.

### TaskSubtasks Adjustments
- [ ] Verify `implementationItems` is correctly iterated.
- [ ] Hide or show a placeholder if `implementationItems` is empty but the task is running.
