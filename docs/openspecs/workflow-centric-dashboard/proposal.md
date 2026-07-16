# Proposal: Workflow-Centric Dashboard Redesign

## Why

An Iteration 2 UX review scored the Task Detail dashboard **7.8/10** overall but rated **Workflow Visibility at 2/5** and **Implementation Tracking at 2/5**. The core finding: the interface is **document-centric** (long spec text, generic metrics) instead of **workflow-centric** (task checklists, live progress, current action).

Users must currently read three separate components (`SpecPanel`, `WorkflowSidebar`, `WorkflowTimeline`) and mentally correlate them to answer basic questions:
- What is the AI doing right now?
- What has been completed?
- What remains?
- Is anything blocked?

The Iteration 1 spec (`task-detail-ui-enhancement`) addressed visual hierarchy, action discoverability, and section ordering — those improvements are **kept**. This Iteration 2 spec addresses the remaining gap: making the **task list** the central element and the specification supporting documentation.

**Success metric**: A user should answer the 6 workflow questions within **5 seconds** of loading the page.

## What Changes

### Issue 1: Description Section Too Large — Collapse by Default
`SpecPanel.tsx` renders the full task description expanded. After the first read it becomes visual noise. Collapse the description by default with a "Show Description" toggle; persist collapse state in `localStorage`.

### Issue 2: Specification as Document → Specification as Checklist
The `SpecPanel` Summary tab renders long paragraphs from `TaskAnalysis`. Convert the `execution_units` from `TaskAnalysis` into an interactive checklist view. Each execution unit maps to a checkbox item showing: unit name, status (done/in-progress/pending), and affected files count.

### Issue 3: No Clear "Current Task" Indicator
Although `ActiveWorkflowBanner` shows the current step (e.g. "code_backend_2"), it doesn't show *what* is being implemented. Add a **Current Implementation Card** that extracts the current execution unit's human-readable description, shows the file currently being modified (from the latest tool call log), and displays elapsed time for the current step.

### Issue 4: Missing Task Progress Visualization
Replace the percentage-based progress bar with a **task-level progress list** grouped by status: ✅ Completed, 🔵 In Progress, ⬚ Pending. Data source: cross-reference `workflow.checkpoints` with `execution_units` from `TaskAnalysis`.

### Issue 5: Workflow Timeline Lacks Implementation Detail
`WorkflowTimeline` shows high-level phases (Analyze → Plan → Code → Review → Fix). Add an **implementation sub-timeline** under the "Code" phase node that lists individual execution units with their completion state. This gives users both the workflow stage AND implementation progress in one view.

### Issue 6: Execution Log Too Technical → Milestone Timeline
The `LogConsole` already groups logs into collapsible step groups. Add a **Milestone Summary** mode (toggle) that filters to show only major events: requirement generated, planning completed, each unit started/completed, waiting for approval. Detailed logs remain available in the existing mode.

### Issue 7: Generic Progress Metrics → Task-Based Metrics
`DashboardSummary` currently shows: Status, Current Step, Progress %, Steps Done, Error Count, Elapsed Time. Replace "Progress %" and "Steps Done" with task-based metrics: Tasks X/Y, Current Unit Name, Remaining Count. Data derivable from `execution_units` + `checkpoints`.

### Issue 8: Agent Activity Lacks Context
`WorkflowSidebar` Agent Activity shows Assigned Agent, Current Step, Attempt, Last Error. Add a **live action indicator** showing the last tool call name and target file from the SSE log stream (e.g. "Editing sync_service.go", "Running tests").

### Issue 9: Missing Persistent Implementation Checklist
Add a dedicated **Implementation Checklist** section (below the timeline, above collapsed spec) that persists across page navigations. Each item is an execution unit with checkbox state derived from checkpoint data. This becomes the primary progress tracker, replacing the spec document for day-to-day monitoring.

### Issue 10: Layout Reordering — Tasks First, Spec Last
Reorder the page layout to prioritize workflow:
```
Task Header
├── Current Status + Current Task + Implementation Progress
├── Task Flow (with implementation sub-timeline)
├── Implementation Checklist
├── Execution Timeline (milestone mode default)
└── Specification (collapsed by default)
```

## Capabilities

### New Capabilities
- `implementation-checklist`: Persistent checklist derived from execution units + checkpoints
- `current-implementation-card`: Shows current unit being worked on with file + elapsed time
- `milestone-timeline-mode`: Filtered log view showing only major events
- `live-action-indicator`: Real-time display of AI's current tool operation

### Modified Capabilities
- `SpecPanel`: Description collapsed by default; checklist view alongside document view
- `DashboardSummary`: Task-based metrics replace percentage metrics
- `WorkflowTimeline`: Implementation sub-timeline under code phase nodes
- `WorkflowSidebar`: Agent Activity shows live action context
- `page.tsx`: Layout reordered to task-first, spec-last

### Removed Capabilities
- None. All Iteration 1 improvements are preserved.

## Impact

| Area | Files Affected |
|------|----------------|
| Page layout | `web/src/app/projects/[id]/tasks/[taskID]/page.tsx` |
| Implementation Checklist (new) | `web/src/app/projects/[id]/tasks/[taskID]/components/ImplementationChecklist.tsx` |
| Current Task Card (new) | `web/src/app/projects/[id]/tasks/[taskID]/components/CurrentImplementationCard.tsx` |
| Milestone Mode (new) | `web/src/components/dashboard/log-console.tsx` |
| SpecPanel | `web/src/app/projects/[id]/tasks/[taskID]/components/SpecPanel.tsx` |
| Dashboard Summary | `web/src/app/projects/[id]/tasks/[taskID]/components/DashboardSummary.tsx` |
| Workflow Timeline | `web/src/app/projects/[id]/tasks/[taskID]/components/WorkflowTimeline.tsx` |
| Workflow Sidebar | `web/src/app/projects/[id]/tasks/[taskID]/components/WorkflowSidebar.tsx` |
| Task Detail Context | `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx` |

## Open Questions
- **Execution unit → checkpoint mapping accuracy**: Checkpoints are step-level (`code_backend_0`, `code_backend_1`, ...) but execution units have arbitrary IDs. Mapping depends on the plan step's `subtask_index` alignment. Needs verification against real data.
- **Live action indicator performance**: Streaming every tool call via SSE may be expensive. Consider throttling to last-action-only or polling at 2s intervals.
- **Milestone event detection**: Log messages don't have semantic tags — milestone detection relies on regex patterns (e.g. `step X success`). May need structured log events in the backend for reliable detection.
