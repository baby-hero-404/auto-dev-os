# Design: Task Detail Layout Rebuild

## Design Philosophy

Transform from **admin dashboard** (collection of equal-weight widgets) → **AI execution console** (narrative flow with clear information hierarchy).

The user's 5 questions, answered by visual hierarchy:

| Question | Answered by | Priority |
|----------|------------|----------|
| What is running? | ExecutionPanel — highlighted current item + live action | **P0** (always visible) |
| What is done / remaining? | ExecutionPanel — checklist with status markers | **P0** |
| Is the AI waiting for me? | ReviewActionBar — conditional sticky bar | **P0** |
| What happened? | CompactTimeline — vertical step history | **P1** (visible but compact) |
| What is the spec / logs / description? | SupportingAccordion — collapsed by default | **P2** (on-demand) |

## Page Composition (New)

```
┌─────────────────────────────────────────────┐
│ TaskHeader (slim)                            │  ← Back, Title, Badges, Run Controls
├─────────────────────────────────────────────┤
│ ReviewActionBar (conditional)               │  ← Only when human action needed
├─────────────────────────────────────────────┤
│ ExecutionPanel                              │  ← PRIMARY: Progress + Checklist + Live
│ ┌─────────────────────────────────────────┐ │
│ │ Progress Row: 60% | 3/5 done | 02:45   │ │
│ ├─────────────────────────────────────────┤ │
│ │ ☑ Database setup                       │ │
│ │ ☑ Repository layer                     │ │
│ │ 🔵 Scheduler service ← Editing: cron.go│ │  ← current item + live action
│ │ ⬜ Retry logic                          │ │
│ │ ⬜ Testing                              │ │
│ └─────────────────────────────────────────┘ │
├─────────────────────────────────────────────┤
│ CompactTimeline (secondary)                 │  ← Vertical, compact step history
│  ● Context Load ............ 2s  ✓         │
│  ● Analyze ................. 15s ✓         │
│  ● Plan .................... 8s  ✓         │
│  ● Implementation .......... 2m  ●         │  ← running
│  ○ Review                                   │
│  ○ PR                                       │
├─────────────────────────────────────────────┤
│ PRPanel (conditional — only when pr_ready)  │
├─────────────────────────────────────────────┤
│ Error/Paused banners (conditional)          │
├─────────────────────────────────────────────┤
│ Supporting Information                      │
│  ▸ Specification .... Scope ✓ Design ✓     │  ← collapsed, summary chips
│  ▸ Execution Logs ... "Step code_be_0 ..."  │  ← collapsed, latest event
│  ▸ Description ...... "Build a task mgr..." │  ← collapsed, one-line truncation
│  ▸ Agent & Checkpoints ... 4 checkpoints   │  ← collapsed, count
└─────────────────────────────────────────────┘
```

## Component Design

### 1. ExecutionPanel.tsx (NEW — replaces DashboardSummary + CurrentImplementationCard + ImplementationChecklist)

**Data sources** (all from `useTaskDetail()`):
- `implementationItems`, `currentImplementationItem` — checklist data
- `workflowCompletion`, `workflowStatusCounts` — progress metrics
- `workflow` — elapsed time, job status
- `logs` — live action parsing (reuse `parseLiveAction` from `CurrentImplementationCard`)
- `displayFiles` — file count

**Structure:**
```tsx
<section> {/* Single card — no inner cards/borders */}
  {/* Row 1: Progress summary — inline, not a separate card */}
  <div className="flex items-center justify-between">
    <ProgressBar percentage />
    <span>3/5 completed</span>
    <span>02:45 elapsed</span>
  </div>

  {/* Row 2: Checklist — the primary content */}
  <div>
    {implementationItems.map(item => (
      <ChecklistRow
        key={item.id}
        isCurrent={item === currentImplementationItem}
        liveAction={isCurrent ? parseLiveAction(logs) : null}
        onClick={() => expandAndScrollToLog(item.stepId)}
      />
    ))}
  </div>
</section>
```

**Key design decisions:**
- Progress row is an inline strip, not a dashboard card with 6 columns.
- Current item is highlighted with a left accent border + live-action badge inline — no separate `CurrentImplementationCard`.
- Items use a flat list layout (not `grid-cols-2`) for scannability — each item is a single row.
- Empty state (pre-analysis): shows a minimal "Waiting for analysis..." state with just status + elapsed.

### 2. CompactTimeline.tsx (NEW — replaces WorkflowTimeline)

**Data sources** (all from `useTaskDetail()`):
- `workflowSteps`, `stepMetadata`, `stepDurations`, `latest` — step status/timing
- `analysisData` — step name formatting

**Structure:**
```tsx
<section> {/* Lightweight card — shadow-sm, not shadow-lg */}
  <h3>Workflow Phases</h3>
  <div className="flex flex-col gap-1">
    {timelineNodes.map(node => (
      <div className="flex items-center gap-3 py-1.5">
        <StatusDot status={nodeStatus} />  {/* 8px dot, not 52px circle */}
        <span>{node.title}</span>
        <span className="ml-auto font-mono text-xs">{duration}</span>
        <StatusIcon />  {/* ✓ or spinner */}
      </div>
    ))}
  </div>
</section>
```

**Key design decisions:**
- Vertical layout, not horizontal rail. No connector lines or hover tooltips.
- 8px status dots (colored) instead of 52px circle nodes.
- Each row: dot → name → flex-grow dotted line → duration → status icon.
- Code steps grouped into a single "Implementation" row (matching the ExecutionPanel's checklist scope).
- Total height: ~200px for 8 steps, vs. ~400px+ for the current horizontal timeline.

### 3. SupportingAccordion.tsx (NEW)

**Structure:**
```tsx
<section>
  <h3>Supporting Information</h3>
  <AccordionItem title="Specification" summary={presenceChips} defaultOpen={false}>
    <SpecPanel isExpanded={true} />  {/* SpecPanel always renders expanded when inside */}
  </AccordionItem>
  <AccordionItem title="Execution Logs" summary={latestLogLine} defaultOpen={false}>
    <LogConsole isExpanded={true} />
  </AccordionItem>
  <AccordionItem title="Description" summary={truncatedDescription} defaultOpen={false}>
    <DescriptionBody />  {/* Extracted from TaskHeader */}
  </AccordionItem>
  <AccordionItem title="Agent & Checkpoints" summary={`${checkpointCount} checkpoints`} defaultOpen={false}>
    <CheckpointsPanel />
  </AccordionItem>
</section>
```

**Key design decisions:**
- Independent collapse (not mutually exclusive) — the user may want Spec + Logs open simultaneously.
- Each accordion header shows a meaningful summary line so users know what's inside without opening.
- The SpecPanel receives `isExpanded={true}` when its accordion is open — it no longer self-manages its outer collapse (its inner tab navigation and collapsible sections remain).
- LogConsole receives the same treatment — accordion controls outer collapse, LogConsole handles its internal grouping.

### 4. TaskHeader.tsx (SLIMMED)

**Remove from TaskHeader:**
- Lines 303-396: the entire "Status strip" section (Current Step, Progress, Assigned Agent, Attempts, Last Error) — all moved to ExecutionPanel.
- Lines 205-299: the collapsible description body and edit-description UX — moved to SupportingAccordion.
- Lines 75-76: `attemptsCount` and `lastError` variables.

**Keep in TaskHeader:**
- Back link (lines 138-144)
- Editable title (lines 146-186)
- Status badges strip (lines 188-203)
- Run controls: Analyze/Execute/Resume/Pause/Cancel/Delete (lines 340-394)
- ConfirmDialog for delete (lines 398-406)

**Expected result:** ~150 LOC, single-purpose component.

## Cross-Component State Coordination

**Expanding log from checklist click:**
```
page.tsx state: [openAccordionSections, setOpenAccordionSections]
                           │
ExecutionPanel ─── onClick(stepId) ─→ page.tsx callback:
  1. setOpenAccordionSections(prev => ({...prev, logs: true}))
  2. requestAnimationFrame(() => scrollTo(`log-group-${stepId}`))
```

This replaces the current `[isLogExpanded, setLogExpanded]` pattern with a more general accordion state map.

## Data Flow Diagram

```
TaskDetailContext (UNCHANGED)
       │
       ├── TaskHeader          reads: task, workflow, projectID, updateTask, execute, analyze, etc.
       ├── ReviewActionBar     reads: task, approveSpec, requestSpecChanges, etc.
       ├── ExecutionPanel      reads: implementationItems, currentImplementationItem, workflow,
       │                              workflowCompletion, logs, displayFiles
       ├── CompactTimeline     reads: workflowSteps, stepMetadata, stepDurations, latest, analysisData
       ├── PRPanel             reads: task, hasPR, diffText, prSummaries, etc.
       └── SupportingAccordion
            ├── SpecPanel      reads: (same as before — no change)
            ├── LogConsole     reads: logs, isWorkflowRunning
            ├── Description    reads: task.description, descriptionParts, updateTask
            └── Checkpoints    reads: workflow.checkpoints
```

## Q&A

**Q: Why not a 2-column layout?**
A: The user explicitly rejected this. The content is execution-oriented, not navigation-oriented. A single-column flow tells a story better.

**Q: Why independent accordion sections instead of mutually exclusive?**
A: Users may want Spec + Logs open simultaneously during a review. Mutually exclusive forces unnecessary clicks.

**Q: What about the `parseLiveAction` function?**
A: It moves from `CurrentImplementationCard.tsx` to `ExecutionPanel.tsx` (or a shared util). Same logic, just relocated.

**Q: Is WorkflowTimeline completely removed?**
A: The file `WorkflowTimeline.tsx` is replaced by `CompactTimeline.tsx`. The new component reuses some of the same data hooks (`workflowSteps`, `latest`, `stepDurations`) but renders a fundamentally different layout.

**Q: What about BoundaryResolutionControls in page.tsx?**
A: The paused/failed banners and `BoundaryResolutionControls` remain in `page.tsx`, positioned between `PRPanel` and `SupportingAccordion` (before the reference surfaces).

**Q: New CSS/tokens for the console look, or existing glassmorphism tokens? (decided)**
A: Existing tokens only — `--stroke`, `--card`, `--surface`, `--brand-primary`, `--success/warning/danger/info` and existing Tailwind utilities. No new custom properties, theme overrides, or shared CSS classes. The one deliberate change: the decorative glass treatment (`backdrop-blur-xl` + gradient hover overlay) currently applied to every card is *removed from secondary/tertiary surfaces* (`CompactTimeline`, accordion rows → plain `bg-card`, `shadow-sm`/flat) and kept only on the primary `ExecutionPanel`. Hierarchy is achieved by subtracting ornament, not adding a new style system.

**Q: Persist accordion open/close state (localStorage) or in-memory? (decided)**
A: In-memory only (`openSections` map in `page.tsx`). REQ-003's first scenario mandates default-collapsed on page load, which persistence would violate on revisits; and a persisted `logs: true` would fight the programmatic `expandAndScrollToLog` flow. Exception preserved: `SpecPanel`'s *inner* localStorage persistence (`task-desc/risks/boundaries-collapsed-${taskID}`) is untouched per the Non-Requirements.

**Q: Which existing tests break? (verified)**
A: `e2e/task-detail.spec.ts` test 1 asserts the old structure ("Show project description", "Implementation Checklist"/"Workflow Phases" headings + ordering, "View details"/"View full log") — rewrite required (Phase 6). Its two sticky-bar tests survive unchanged (ReviewActionBar untouched). `e2e/mapping.spec.ts` survives — it tests `deriveImplementationItems` from the unchanged `TaskDetailContext`. There is no Jest/Vitest unit-test surface in `web/`.
