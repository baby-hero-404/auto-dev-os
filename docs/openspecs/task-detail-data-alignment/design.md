# Design: Task Detail Data Alignment

## Architecture Adjustments

The modular architecture of the Task Detail page will be retained (using `TaskDetailLayout`, `TaskHeroCards`, `TaskSidebar`, etc.), but the internal data mapping logic within these components will be adapted.

### TaskHeroCards.tsx
- Replace mock data placeholders with actual bindings to `analysisData` (e.g. `analysisData.proposal_md`, `analysisData.specs_md`, `analysisData.design_md`, `analysisData.tasks_md`).
- For execution logging, map over the actual `logs` array provided by `useTaskDetail()`. Implement a basic auto-scroll or inverse render to ensure the latest log is always visible without blowing up the layout bounds.

### TaskSidebar.tsx
- The workflow stepper currently uses a hardcoded `phaseDefs` array mapping to arbitrary task statuses.
- This will be replaced by reading `workflowSteps` directly from the context.
- We will map over `workflowSteps` and look up the status in the `latest` map to determine whether a step is `completed`, `running`, or `pending`.
- Step formatting will utilize `formatStepName` and `getSemanticStatusColor` from `TaskDetailContext.tsx` if appropriate, blending the new visual styling with the correct dynamic data.

### TaskSubtasks.tsx
- The progress bar logic will directly count items from `implementationItems` (checking if `item.status === 'done'`).
- The rendering loop will use real `item.name` and handle edge cases where `implementationItems` is empty by falling back gracefully or rendering a placeholder.
