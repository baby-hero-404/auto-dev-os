# Tasks: Workflow-Centric Dashboard Redesign

## Phase 1: Data Layer — Execution Unit Mapping (Priority: ⭐⭐⭐⭐⭐)

- [x] **1.1** Add `deriveImplementationItems()` helper to `TaskDetailContext.tsx`
  - Maps `execution_units[]` + `checkpoints[]` → `ImplementationItem[]`
  - Expose via context: `implementationItems`, `currentImplementationItem`
  - Memoize with `useMemo`
- [x] **1.2** Add unit tests for the mapping logic
  - Test: all units done → all status "done"
  - Test: middle unit running → correct "running" + "pending" split
  - Test: no execution_units → empty list (graceful fallback)
  - Test: checkpoint names don't match → all "pending"

## Phase 2: Implementation Checklist Component (Priority: ⭐⭐⭐⭐⭐)

- [x] **2.1** Create `ImplementationChecklist.tsx`
  - Renders `implementationItems` grouped by status
  - Checkbox icons: ✅ done, 🔵 running (pulse), ⬚ pending
  - Shows affected files count per item
  - Click item scrolls to relevant log group (if visible)
- [x] **2.2** Integrate into `page.tsx layout`
  - Position: below WorkflowTimeline, above LogConsole
  - Only renders when `implementationItems.length > 0`

## Phase 3: Current Implementation Card (Priority: ⭐⭐⭐⭐⭐)

- [x] **3.1** Create `CurrentImplementationCard.tsx`
  - Shows: unit name, "In Progress" badge with pulse, elapsed time, current file
  - Current file: extracted from latest log entry matching tool call pattern
  - Only visible when `workflow.job.status === 'running'`
- [x] **3.2** Integrate into `DashboardSummary.tsx`
  - Replace or supplement the generic "Current Step" metric

## Phase 4: Description Collapse (Priority: ⭐⭐⭐⭐⭐)

- [x] **4.1** Make SpecPanel description collapsed by default
  - Add collapse state with localStorage persistence: `task-desc-collapsed-${taskID}`
  - Animated expand/collapse with CSS `max-height` transition
  - Collapsed view: first 2 lines with gradient fade + "Show Description" link
- [x] **4.2** Apply same collapse pattern to Risks and Execution Boundaries
  - These were marked collapsible in Iteration 1 spec but need consistent implementation

## Phase 5: DashboardSummary Metric Overhaul (Priority: ⭐⭐⭐⭐☆)

- [x] **5.1** Replace percentage metrics with task-based metrics
  - "Tasks: X / Y" (from `implementationItems`)
  - "Current: {unit name}" instead of step ID
  - "Remaining: N"
- [x] **5.2** Update progress bar to reflect task completion
  - `completedTasks / totalTasks` ratio instead of `workflowCompletion`

## Phase 6: Workflow Timeline Sub-Items (Priority: ⭐⭐⭐⭐☆)
* **Status: Completed**

- [x] **6.1** Add implementation sub-timeline under code phase nodes
  - Each `code_backend_N` / `code_frontend_N` step gets a sub-list of execution units
  - Sub-items show: unit name + status icon
  - Running sub-item has pulse animation matching the parent node
- [x] **6.2** Make sub-timeline collapsible
  - Expanded by default during execution
  - Collapsed after workflow completion

## Phase 7: Milestone Timeline Mode (Priority: ⭐⭐⭐⭐☆)
* **Status: Completed**

- [x] **7.1** Add Milestones/All Logs toggle to `log-console.tsx`
  - Tab-style toggle at top of component
  - Default: "Milestones" when running, "All Logs" when complete
- [x] **7.2** Implement milestone filtering logic
  - Regex patterns: `/step (\w+) (success|failed|running)/`, `/checkpoint/`, `/paused|resumed/`, `/workflow (failed|completed)/`
  - Render as vertical timeline with icons + timestamps
  - Color-coded: green (success), blue (running), red (error), gray (info)

## Phase 8: Live Action Indicator (Priority: ⭐⭐⭐☆☆)
* **Status: Completed**

- [x] **8.1** Add `LiveActionIndicator` to `WorkflowSidebar.tsx` Agent Activity section
  - Extract last tool call from SSE stream: tool name + target file
  - Display: icon + "Editing sync_service.go" / "Running tests" / "Thinking..."
  - Throttle updates to 1s intervals
- [x] **8.2** Parse tool call patterns from log messages
  - Pattern: `search_replace`, `create_file` → "Editing {file}"
  - Pattern: `run_tests`, `run_build`, `run_lint` → "Running {tool}"
  - Pattern: `read_file`, `list_files` → "Reading {file}"
  - Fallback: "Processing..."

## Phase 9: Layout Reordering (Priority: ⭐⭐⭐☆☆)
* **Status: Completed**

- [x] **9.1** Reorder `page.tsx` sections
  - New order: Header → DashboardSummary → Banner → PRPanel → Timeline → Checklist → LogConsole → SpecPanel
  - SpecPanel moves to bottom (collapsed by default)
- [x] **9.2** Verify responsive behavior
  - Mobile: all sections stack vertically
  - Sidebar accordion behavior preserved from Iteration 1

## Phase 10: E2E Tests (Priority: ⭐⭐⭐☆☆)
* **Status: Completed**

- [x] **10.1** Update `task-detail.spec.ts` with new component tests
  - Test: ImplementationChecklist renders with execution units
  - Test: CurrentImplementationCard shows during running workflow
  - Test: Description collapsed by default
  - Test: Milestone mode toggle in LogConsole
- [x] **10.2** Update `api-mocks.ts` with execution_units mock data
  - Add realistic execution_units to the task mock analysis
  - Add corresponding checkpoint mocks

---

## Dependencies
- **Phase 1** must complete before Phases 2, 3, 5, 6 (all depend on `implementationItems`)
- **Phase 4** is independent and can be done in parallel with Phase 1
- **Phase 7, 8** are independent of the execution unit mapping
- **Phase 9** should be done after Phases 2-4 are integrated
- **Phase 10** should be done last

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
