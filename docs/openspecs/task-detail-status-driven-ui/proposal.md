# Proposal: Status-Driven Task Detail UI

## Why
The current Task Detail page presents a lot of information simultaneously, which can overwhelm the user. Because a task progresses through multiple distinct phases (Todo -> Spec Review -> Execution -> PR -> Merged/Failed), the user's focus shifts at each step. By transitioning to a **Status-Driven UI**, we can conditionally display or prioritize only the information that is immediately relevant to the current state, reducing cognitive load and creating a more guided, action-oriented experience.

## What Changes

### Issue 1: Contextual Information Hierarchy
- Implement a state machine for the UI that maps `task.status` to visible components.
- Define explicit visibility rules for every panel (Hero Cards, Subtasks, Logs, Supporting Accordion) based on the active status.

### Issue 2: Description Placement & Editing
- Move the Task Description out of the bottom accordion or isolated cards, and place it directly beneath the Task Title.
- Make the description collapsible (defaulting to a truncated view) so it provides immediate context without pushing critical execution data below the fold.
- Retain the ability to edit the description in this new location.

### Issue 3: Execution vs. Historical Context
- Ensure the upper section of the page ("Hero" section) acts as the active execution console (showing what is happening *now*).
- Ensure the lower section (`SupportingAccordion`) acts as the historical/reference library (allowing access to past logs, full specs, and checkpoints even after the task is merged).

## Capabilities

### New Capabilities
- **Status-Driven Component Routing:** UI components will intelligently collapse or expand depending on whether the task is in planning, execution, or finalization.

### Modified Capabilities
- **Description Viewer:** Relocated to the Title Block and enhanced with collapsible formatting.

### Removed Capabilities
- Redundant "Description" blocks scattered across the page.

## Impact

| Area | Files Affected |
|------|----------------|
| Layout | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailLayout.tsx` |
| Title Block | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskTitleBlock.tsx` |
| Hero Cards | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx` |
| Description | `web/src/app/projects/[id]/tasks/[taskID]/components/DescriptionBody.tsx` |
