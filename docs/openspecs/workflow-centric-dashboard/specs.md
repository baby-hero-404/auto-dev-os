# Specs: Workflow-Centric Dashboard Redesign

## Added Requirements

### REQ-001: Collapsible Description
> ✅ Status: Completed

**Scenario:**
- WHEN a user opens the Task Detail page
- THEN the project description section is collapsed by default
- AND a "Show Description" toggle is visible
- AND clicking the toggle expands the full description
- AND the collapse state persists across page navigations via localStorage

### REQ-002: Implementation Checklist View
> ✅ Status: Completed

**Scenario:**
- WHEN a task has execution_units in its TaskAnalysis
- THEN an Implementation Checklist section renders below the workflow timeline
- AND each execution unit appears as a checkbox item with: name, status icon (✅/🔵/⬚), affected files count
- AND the status is derived from cross-referencing checkpoints with execution unit indices
- AND completed items show a checkmark, in-progress items show a blue dot, pending items show an empty box

### REQ-003: Current Implementation Card
> ✅ Status: Completed

**Scenario:**
- WHEN a workflow job is running
- THEN a "Current Implementation" card is visible in the dashboard summary area
- AND it shows the current execution unit's human-readable name
- AND it shows the status as "In Progress" with a pulsing indicator
- AND it shows the elapsed time since the current step started
- AND it shows the current file being modified (from the latest log entry with a tool call)

### REQ-004: Task-Level Progress List
> ✅ Status: Completed

**Scenario:**
- WHEN viewing the dashboard summary
- THEN progress is shown as a grouped task list (Completed / In Progress / Pending)
- AND each group shows the count of items
- AND the task names are human-readable execution unit descriptions
- AND percentage-based progress is replaced or supplemented with "X / Y tasks"

### REQ-005: Implementation Sub-Timeline
> ❌ Status: Not Started

**Scenario:**
- WHEN the WorkflowTimeline renders the "Code" phase node(s)
- THEN an expandable sub-timeline appears under each code phase
- AND each sub-item represents an execution unit
- AND sub-items show completion state (done/running/pending)
- AND the currently running sub-item has a pulse animation

### REQ-006: Milestone Timeline Mode
> ❌ Status: Not Started

**Scenario:**
- WHEN the LogConsole component renders
- THEN a "Milestones" / "All Logs" toggle is available
- AND the Milestones mode shows only major events: step started, step completed, errors, waiting for approval
- AND detailed tool call logs are hidden in Milestones mode
- AND the All Logs mode preserves the existing collapsible step group behavior

### REQ-007: Task-Based Dashboard Metrics
> ✅ Status: Completed

**Scenario:**
- WHEN DashboardSummary renders
- THEN it shows "Tasks: X / Y" instead of or alongside "Progress: X%"
- AND it shows the current execution unit name (e.g. "Sync Service") instead of just step ID
- AND it shows "Remaining: N" as a count of pending execution units

### REQ-008: Live Action Indicator
> ❌ Status: Not Started

**Scenario:**
- WHEN a workflow is actively running
- THEN the Agent Activity section shows the last tool call action (e.g. "Editing sync_service.go")
- AND it updates in near-real-time via SSE log stream
- AND when no tool is actively running, it shows "Thinking..." or the current phase name

### REQ-009: Persistent Checklist Navigation
> ✅ Status: Completed

**Scenario:**
- WHEN a user navigates away from the task detail and returns
- THEN the Implementation Checklist state is preserved from checkpoint data
- AND no client-side state is required (all derived from server-side checkpoint + analysis data)

### REQ-010: Layout Reordering
> ❌ Status: Not Started

**Scenario:**
- WHEN the Task Detail page renders
- THEN the section order from top to bottom is:
  1. Task Header (with status strip)
  2. Dashboard Summary (with current task + task-based metrics)
  3. Active Workflow Banner (conditional)
  4. Workflow Timeline (with implementation sub-timeline)
  5. Implementation Checklist
  6. Execution Timeline / Log Console (milestone mode default)
  7. Specification Panel (collapsed by default)
- AND this represents a shift from document-first to workflow-first layout

## Modified Requirements

### REQ-M01: SpecPanel Collapse Enhancement
> ✅ Status: Completed

**Scenario:**
- WHEN SpecPanel's Summary tab renders
- THEN the description, risks, and execution boundaries are all collapsed by default
- AND each section has a toggle to expand
- AND a new "Checklist View" tab is added alongside Summary/Proposal/Specs/Design/Tasks
- AND the Checklist View renders execution units as an interactive list

### REQ-M02: DashboardSummary Metric Overhaul
> ✅ Status: Completed

**Scenario:**
- WHEN DashboardSummary has execution_units data available from TaskAnalysis
- THEN "Steps Done / Total" becomes "Tasks Done / Total" with execution unit granularity
- AND "Current Step" shows the execution unit name instead of the workflow step ID
- AND a mini progress bar reflects task completion, not step completion

## Removed Requirements
- None. All prior Iteration 1 requirements remain in effect.
