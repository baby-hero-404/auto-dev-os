# Design: Status-Driven Task Detail UI

## Architectural Approach

The layout heavily depends on the `task.status` state derived from `TaskDetailContext`. We split the presentation into distinct zones:

1. **Top Zone (`TaskHeader` & `TaskTitleBlock`)**:
   - Universal context available in all statuses.
   - Includes breadcrumb (`← Back to Project`), Task Title, Status Badges, Priority, and the newly integrated inline Description block.

2. **Hero Zone (`TaskHeroCards`)**:
   - This is the highly volatile *Status-Driven* area.
   - It acts as a router, rendering completely different sub-components based on `st` (task status).
   - If `st === 'todo'`, displays initial info.
   - If `st === 'spec_review'`, renders the `Markdown` spec component.
   - If `st` is in execution (`coding`, `testing`), renders the `LogConsole` directly.
   - If `st === 'failed'`, renders the error along with the embedded `LogConsole`.

3. **Supporting Zone (`SupportingAccordion`)**:
   - The fallback state holder.
   - It is persistently available at the bottom of `TaskDetailLayout`.
   - Contains components like `SpecPanel`, `LogConsole` (in an accordion), `DescriptionBody` (for editing), and `CheckpointsPanel`.
   - Ensures that moving to a Status-Driven upper UI does not destroy the ability to view historical data for completed or failed tasks.

## Data Flow
- `TaskDetailContext` remains the single source of truth for both `task.status` and `workflowSteps`.
- Components are stateless regarding the workflow; they only react to the props and context provided by `TaskDetailContext`.
- We utilize `@/components/ui/markdown` instead of raw `react-markdown` to ensure standard typography patterns (`prose`) across both the Hero Card and Supporting Accordion.
