# Design: Workflow-Centric Dashboard Redesign

## Architecture Overview

This redesign is **frontend-only** — no backend API changes are required. All data needed for the new views is already available through existing `TaskDetailContext` providers:
- `TaskAnalysis.execution_units` → checklist items
- `workflow.checkpoints` → completion state per step
- `workflow.job` → current running state
- SSE log stream → live action indicator
- `stepDurations` → elapsed time calculations

## Data Flow

```
TaskDetailContext (existing)
├── task.analysis.execution_units[] ──→ ImplementationChecklist
│                                       CurrentImplementationCard
│                                       DashboardSummary (task metrics)
│                                       WorkflowTimeline (sub-items)
├── workflow.checkpoints[] ───────────→ Completion state mapping
├── workflow.job.status ──────────────→ ActiveWorkflowBanner
├── workflowCompletion ───────────────→ DashboardSummary
└── SSE log stream ───────────────────→ LiveActionIndicator
                                        MilestoneTimeline
```

## Execution Unit → Checkpoint Mapping

The key challenge is mapping `execution_units[i]` to checkpoint completion. The current system creates checkpoints named `code_backend_0`, `code_backend_1`, etc., where the suffix index corresponds to the execution unit's position in the plan's subtask list.

### Mapping Strategy
```typescript
// In TaskDetailContext, derive a merged list:
interface ImplementationItem {
  id: string;              // execution_unit.id
  name: string;            // execution_unit title/capability
  description: string;     // human-readable summary
  status: 'done' | 'running' | 'pending';
  affectedFiles: string[];
  stepId: string;          // e.g. "code_backend_2"
  checkpointExists: boolean;
}

function deriveImplementationItems(
  analysis: TaskAnalysis,
  checkpoints: WorkflowCheckpoint[],
  currentStep: string
): ImplementationItem[] {
  const completedSteps = new Set(
    checkpoints.filter(cp => cp.step.startsWith('code_'))
               .map(cp => cp.step)
  );
  
  return analysis.execution_units.map((unit, idx) => {
    const role = unit.execution_profile.agent; // "backend" or "frontend"
    const stepId = `code_${role}_${idx}`;
    const isDone = completedSteps.has(stepId);
    const isRunning = currentStep === stepId;
    
    return {
      id: unit.id,
      name: unit.title || unit.intent?.capability || `Unit ${idx}`,
      description: unit.description || '',
      status: isDone ? 'done' : isRunning ? 'running' : 'pending',
      affectedFiles: unit.affected_files || [],
      stepId,
      checkpointExists: isDone,
    };
  });
}
```

## New Components

### 1. `ImplementationChecklist.tsx`
- **Purpose**: Primary progress tracker
- **Data source**: `deriveImplementationItems()` from context
- **Rendering**: Grouped by status (done → running → pending)
- **Interactivity**: Click an item to scroll to its relevant log group
- **Styling**: Checkbox icons with semantic colors (green/blue/gray)

### 2. `CurrentImplementationCard.tsx`
- **Purpose**: "What is the AI doing right now?" answer
- **Data source**: Current running `ImplementationItem` + latest log message
- **Rendering**: Card with: unit name, status badge, elapsed timer, current file
- **Visibility**: Only shown when `workflow.job.status === 'running'`

### 3. `MilestoneTimeline` (within LogConsole)
- **Purpose**: Non-technical progress view
- **Detection**: Regex patterns on log messages:
  - `/step (\w+) (success|failed|running)/` → step milestone
  - `/checkpoint/` → progress saved
  - `/paused|resumed/` → workflow state change
  - `/workflow failed|completed/` → terminal event
- **Rendering**: Vertical timeline with icons and timestamps

### 4. `LiveActionIndicator` (within WorkflowSidebar)
- **Purpose**: Real-time AI activity context
- **Data source**: Last SSE log entry matching tool call patterns
- **Detection**: Extract tool name + target from log message content
- **Rendering**: Single-line indicator with icon + action text

## Layout Changes

### Current Layout (`page.tsx`)
```
TaskHeader
DashboardSummary
ActiveWorkflowBanner
PRPanel (conditional)
┌─────────────────────┬──────────────┐
│ WorkflowTimeline    │ Sidebar      │
│ SpecPanel / LogTab  │  - Actions   │
│                     │  - Agent     │
│                     │  - Checkpts  │
└─────────────────────┴──────────────┘
```

### New Layout
```
TaskHeader
DashboardSummary (task-based metrics + CurrentImplementationCard)
ActiveWorkflowBanner
PRPanel (conditional)
┌─────────────────────┬──────────────┐
│ WorkflowTimeline    │ Sidebar      │
│  └─ Sub-timeline    │  - Actions   │
│ ImplementationList  │  - Agent +   │
│ LogConsole          │    LiveAction │
│ SpecPanel(collapsed)│  - Checkpts  │
└─────────────────────┴──────────────┘
```

## Styling Decisions

### Checklist Item States
| State | Icon | Color | Background |
|-------|------|-------|------------|
| Done | `✓` circle | `text-emerald-400` | `bg-emerald-500/10` |
| Running | Pulse dot | `text-blue-400` | `bg-blue-500/10` |
| Pending | Empty circle | `text-muted-foreground` | none |

### Milestone vs Detail Log Toggle
- Tab-style toggle at the top of LogConsole
- Default to "Milestones" when a workflow is running
- Default to "All Logs" when workflow is complete (for debugging)

### Description Collapse
- `localStorage` key: `task-desc-collapsed-${taskID}`
- Animated height transition using CSS `max-height` + `overflow: hidden`
- Collapsed state shows first 2 lines with gradient fade

## Performance Considerations
- `deriveImplementationItems()` memoized with `useMemo` on `[analysis, checkpoints, currentStep]`
- `LiveActionIndicator` throttled to update at most every 1s (not every SSE message)
- `MilestoneTimeline` pre-filters log entries on arrival, not on render
