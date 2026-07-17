# Proposal: Task Detail Layout Rebuild — AI Execution Console

## Why

The task detail page has grown organically through multiple redesign iterations but accumulated three structural problems that incremental fixes cannot address:

1. **Information duplication**: Current step / progress / implementation status is rendered in 4 separate cards (`DashboardSummary`, `CurrentImplementationCard`, `ImplementationChecklist`, `WorkflowTimeline`). Users must mentally merge them.

2. **No information hierarchy**: Every section is an equally-weighted glassmorphism card with title, border, padding, and shadow. Nothing visually reads as more important — the page looks like a dashboard widget collection instead of an execution narrative.

3. **Secondary information competes with primary**: Description (inside TaskHeader — 410 LOC), Specification (SpecPanel — 468 LOC), Logs (LogConsole), and agent metadata all consume the main viewport by default. They should be reference surfaces, not primary content.

The result: users cannot answer the 5 core questions within 3 seconds:
> What is running? What is done? What remains? Is the AI waiting for me? Do I need to act?

## What Changes

### Issue 1: Merge 4 overlapping execution surfaces into 1 unified Execution Panel
- **Remove** `DashboardSummary.tsx` as a standalone component.
- **Remove** `CurrentImplementationCard.tsx` as a standalone component.
- **Absorb** their data into a new `ExecutionPanel.tsx` that contains: (a) an inline progress bar with key metrics, (b) the implementation checklist with the current item highlighted, (c) the live-action indicator (from `CurrentImplementationCard`).
- No new data hooks — all consumed from existing `useTaskDetail()`.

### Issue 2: Compact the TaskHeader — strip the bloat
- **Remove** the full workflow controls strip from TaskHeader (progress %, current step, assigned agent — all now in `ExecutionPanel`).
- **Remove** the inline description editor and collapsible description body — description moves to the "Supporting Information" accordion group.
- **Keep** only: back link, title (editable), status badges, and run controls (Analyze/Execute/Pause/Cancel/Delete).
- Expected reduction: 410 LOC → ~150 LOC.

### Issue 3: Compact the WorkflowTimeline
- Reduce from a full-width horizontal rail with 52px nodes, sub-task trees, and hover tooltips to a **compact vertical timeline** (similar to GitHub Actions sidebar).
- Visual weight drops from primary (`shadow-lg`, 469 LOC) to secondary.

### Issue 4: Collapse all reference surfaces by default
- Specification, Logs, Agent Details, and Checkpoints become an **accordion group** ("Supporting Information") at the bottom of the page. Only one opens at a time (or optionally independent).
- The collapsed summary line for each shows a meaningful one-liner (e.g. latest log event, spec presence chips).

### Issue 5: Review action bar remains — no change
- `ReviewActionBar` is already conditional and well-positioned. No structural change.

## Capabilities

### New Capabilities
- **ExecutionPanel**: Unified execution surface merging progress, checklist, and live action.
- **SupportingAccordion**: Collapsible group for reference surfaces (Spec, Logs, Description, Agent info).
- **CompactTimeline**: Vertical, compact workflow timeline replacing the horizontal rail.

### Modified Capabilities
- **TaskHeader**: Slimmed to title + status + run controls only.
- **ReviewActionBar**: Unchanged in behavior; positioned between header and execution panel.

### Removed Capabilities
- **DashboardSummary**: Data absorbed into ExecutionPanel.
- **CurrentImplementationCard**: Live-action indicator absorbed into ExecutionPanel.
- **Horizontal WorkflowTimeline**: Replaced by CompactTimeline.

## Impact

| Area | Files Affected |
|------|----------------|
| Page composition | `page.tsx` |
| New components | `ExecutionPanel.tsx`, `CompactTimeline.tsx`, `SupportingAccordion.tsx` (+ small `CheckpointsPanel`) |
| Slimmed components | `TaskHeader.tsx` |
| Removed components | `DashboardSummary.tsx`, `CurrentImplementationCard.tsx`, `ImplementationChecklist.tsx` (absorbed into `ExecutionPanel`) |
| Restructured components | `WorkflowTimeline.tsx` → replaced by `CompactTimeline.tsx` |
| Retained as-is | `ReviewActionBar.tsx`, `SpecPanel.tsx`, `PRPanel.tsx`, `RequestChangesModal.tsx`, `TaskDetailContext.tsx` |
| Tests | `e2e/task-detail.spec.ts` |

## Open Questions
- **Accordion behavior**: should the supporting info sections be mutually exclusive (only one open) or independent? Recommendation: independent (multiple can be open simultaneously) — they serve different audiences.
- **Description location**: currently inside TaskHeader. Moving it to the SupportingAccordion requires ensuring the edit-in-place UX still works. Alternatively, keep a one-line truncated description in the header and move the full view to the accordion.
