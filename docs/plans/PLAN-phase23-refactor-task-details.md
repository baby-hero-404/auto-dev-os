# Plan - Refactor Task Detail Page and Clarification Integration

This plan outlines the restructuring of `ProjectTaskDetailPage` (`web/src/app/projects/[id]/tasks/[taskID]/page.tsx`) to address performance, maintainability, clean architecture, and complete the Option A structured task clarification flow.

## 1. Objectives

- **Decompose Component**: Break down the 1500+ line `ProjectTaskDetailPage` into distinct, focused, and testable sub-components.
- **Context-based State**: Introduce `TaskDetailContext` to solve prop drilling and minimize redundant rendering.
- **Structured Q&A Integration**: Integrate `ClarificationPanel` to view structured historical rounds from `task.clarifications`.
- **Workflow Step Enums**: Replace magic strings for workflow steps with clear constants.
- **Performance & Cleanup**: Eliminate unnecessary `useMemo` hooks, safely parse inputs (e.g., `Number()` instead of `parseInt()`), remove inline callbacks using `useCallback` where needed, and fix `replaceAll` / `replace` regex bugs.

## 2. Component Structure

We will create a sub-directory `web/src/app/projects/[id]/tasks/[taskID]/components` to keep the sub-components encapsulated:
1. `TaskDetailContext.tsx` - Holds task data, projectID, SWR mutations, active tab states, and actions.
2. `TaskHeader.tsx` - Renders title, breadcrumbs, editing states, and metadata badges.
3. `WorkflowTimeline.tsx` - Renders the timeline of checkpoints.
4. `SpecPanel.tsx` - Renders the Proposed Task Specification tabs (Summary, Proposals, Design, Tasks).
5. `PRPanel.tsx` - Renders PR status and human review/reject triggers.
6. `WorkflowSidebar.tsx` - Renders progress indicator, checkpoints summary, and action panel.
7. `ClarificationPanel.tsx` - Renders the new structured QA history from `task.clarifications`.

## 3. Implementation Phases

### Phase 1: Context Definition
Create `TaskDetailContext.tsx` with hooks for retrieving state and actions safely.

### Phase 2: Define Constants & Helper Guards
Create a `constants.ts` or add to `task-utils.ts` for step names, avoiding duplicate calculations.

### Phase 3: Extract Sub-components
Develop each sub-component by extracting the corresponding JSX blocks from `page.tsx`.

### Phase 4: Integrate in main `page.tsx`
Replace the inline JSX in `page.tsx` with the provider and sub-components. Validate layout aesthetics.
