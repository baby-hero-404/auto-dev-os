# Proposal: Task Detail Data Alignment

## Why
The recent UI refactor for the Task Detail page successfully implemented the new "task-driven" design system (from `TaskDetail.dc.html`). However, the strict adherence to the static HTML template led to some data mismatch with the actual backend state provided by `useTaskDetail()`. To make the UI truly functional, we need to adapt the design to perfectly fit the real data structures (`Task`, `WorkflowStatus`, `TaskAnalysis`, etc.) while maintaining the core task-driven workflow philosophy. The UI does not need to be 100% identical to the design mock if the data suggests a better approach.

## What Changes

### Issue 1: Data-UI Mismatch in Hero Cards
- Update `TaskHeroCards.tsx` to handle missing data gracefully (e.g., when `analysisData.proposal_md` is empty).
- Ensure log streaming in the execution state maps correctly to real-time logs array and scrolls appropriately.

### Issue 2: Workflow Stepper Accuracy
- Update `TaskSidebar.tsx` to map the workflow timeline to the actual steps returned by `workflowSteps` and `latest` states from the context, rather than hardcoded phase definitions.

### Issue 3: Subtask Mapping
- Ensure `TaskSubtasks.tsx` renders `implementationItems` reliably, handling cases where execution units are not yet defined.

## Capabilities

### Modified Capabilities
- `TaskHeroCards`: Adaptive rendering based on real `TaskAnalysis` and `Workflow` state.
- `TaskSidebar`: Accurate timeline reflecting the true state machine of the backend.
- `TaskSubtasks`: Real progress calculation based on execution units.

## Impact

| Area | Files Affected |
|------|----------------|
| UI Components | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx` |
| UI Components | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSidebar.tsx` |
| UI Components | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSubtasks.tsx` |
| UI Components | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskTitleBlock.tsx` |
