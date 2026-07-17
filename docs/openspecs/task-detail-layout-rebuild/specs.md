# Specs: Task Detail Layout Rebuild

## Added Requirements

### REQ-001: ExecutionPanel replaces 4 overlapping components
> ✅ Status: Completed

**Scenario: unified execution view**
- WHEN the page loads and `implementationItems` is non-empty
- THEN a single `ExecutionPanel` renders containing: (a) a progress summary row (% complete, done/total count, elapsed time), (b) the implementation checklist with the current item visually highlighted and a live-action indicator (Editing/Reading/Running from log stream parsing), (c) pending/completed items below
- AND no separate `DashboardSummary`, `CurrentImplementationCard`, or standalone `ImplementationChecklist` component appears in the DOM.

**Scenario: empty state (pre-analysis)**
- WHEN `implementationItems` is empty (no analysis yet)
- THEN `ExecutionPanel` renders a minimal state showing only: status badge, current workflow step, elapsed time
- AND no "Implementation Checklist" section appears.

**Scenario: live action indicator**
- WHEN the workflow is running
- THEN the currently-running checklist item shows an inline live-action badge (e.g. "Editing: schema.go", "Running: tests") parsed from the latest log entries
- AND the badge updates in real time as new logs arrive.

### REQ-002: CompactTimeline replaces horizontal WorkflowTimeline
> ✅ Status: Completed

**Scenario: compact vertical layout**
- WHEN the page loads with workflow steps
- THEN a `CompactTimeline` renders as a compact vertical list (not a horizontal rail), using small status dots, step name, and duration on each row
- AND no horizontal connector lines, 52px nodes, or hover tooltip cards appear.

**Scenario: timeline answers "what happened"**
- WHEN a user looks at the timeline
- THEN each completed step shows: icon, name, duration, and status
- AND running steps show a pulsing indicator
- AND the total vertical space consumed is ≤ 300px for a typical 8-step workflow.

### REQ-003: SupportingAccordion collapses reference surfaces
> ✅ Status: Completed

**Scenario: default collapsed**
- WHEN the page loads
- THEN Specification, Logs, Description, and Checkpoints each render as a collapsed accordion row showing only a summary line
- AND no reference surface body is visible by default.

**Scenario: expand on demand**
- WHEN the user clicks an accordion row header
- THEN that section expands to show full content (SpecPanel body, LogConsole body, description markdown, checkpoint list)
- AND other accordion sections remain in their current state (independent, not mutually exclusive).

**Scenario: log latest-event summary**
- WHEN the LogConsole accordion is collapsed
- THEN the collapsed header shows the latest log event line with semantic coloring (error=red, info=default)
- AND if there is a recent error, the header shows a warning indicator.

### REQ-004: TaskHeader slimmed to essentials
> ✅ Status: Completed

**Scenario: header content**
- WHEN the page loads
- THEN TaskHeader contains only: back link, editable title, status badges (task status, spec status, job status), and contextual run controls (Analyze/Execute/Pause/Cancel/Delete)
- AND no progress percentage, current step, assigned agent, elapsed time, or description body appears in the header.

**Scenario: description moved**
- WHEN a user wants to read the project description
- THEN they click "Description" in the SupportingAccordion
- AND the description content (with edit capability) expands there — not in the header.

## Modified Requirements

### REQ-M01: ImplementationChecklist interaction preserved
> ✅ Status: Completed

**Scenario: click-to-log preserved**
- WHEN a user clicks a checklist item inside ExecutionPanel
- THEN the SupportingAccordion's Log section expands and scrolls to the matching `log-group-{stepId}` anchor
- AND the interaction uses the same `expandAndScrollToLog` pattern from the previous design.

### REQ-M02: PRPanel positioning
> ✅ Status: Completed

**Scenario: PR panel appears contextually**
- WHEN `task.status === "pr_ready"` or review is active
- THEN `PRPanel` renders between the CompactTimeline and SupportingAccordion (above reference surfaces, below execution content)
- AND its internal structure is unchanged.

## Removed Requirements
- REQ-R01: `DashboardSummary` as a standalone component (data absorbed into ExecutionPanel).
- REQ-R02: `CurrentImplementationCard` as a standalone component (live-action indicator absorbed into ExecutionPanel).
- REQ-R03: Horizontal `WorkflowTimeline` with 52px nodes, connector lines, sub-task trees, and hover tooltips (replaced by CompactTimeline).
- REQ-R04: Description/Clarification body inside TaskHeader (moved to SupportingAccordion).
- REQ-R05: Workflow metrics strip in TaskHeader (progress %, current step, assigned agent, attempts, last error — moved to ExecutionPanel).

## Non-Requirements
- **No backend changes.** All data comes from existing `useTaskDetail()` hooks.
- **No TaskDetailContext changes.** The context, hooks, state management, and API calls are unchanged.
- **No SpecPanel internal redesign.** SpecPanel's tab system and collapsible sections are retained. Only its *mount point* changes (inside SupportingAccordion instead of standalone).
- **No ReviewActionBar changes.** It is already conditional and well-positioned.
