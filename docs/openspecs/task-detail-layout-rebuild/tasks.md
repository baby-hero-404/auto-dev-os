# Tasks: Task Detail Layout Rebuild — AI Execution Console

> State tracked via checkboxes. Phases ordered so the tree typechecks after each phase: build new components first (1–3), slim the header (4), then recompose the page and delete the absorbed components in one move (5), then tests (6).
> Verified starting point (current LOC): `TaskHeader` 409, `WorkflowTimeline` 468, `SpecPanel` 467, `DashboardSummary` 185, `ImplementationChecklist` 139, `CurrentImplementationCard` 137.

## Phase 0: Inherited foundation (from `task-detail-workflow-redesign` — already in the codebase)
- [x] `ReviewActionBar.tsx` — conditional sticky decision bar; retained unchanged (Issue 5).
- [x] `SpecPanel` and `LogConsole` already accept controlled `isExpanded` + `onToggle` props — the accordion reuses this seam instead of adding a new one.
- [x] `expandAndScrollToLog(stepId)` pattern (state in `page.tsx` + `requestAnimationFrame` scroll) — generalized in Phase 5, not invented.
- [x] `implementationItems` / `currentImplementationItem` / `parseLiveAction` / `workflowCompletion` / `stepDurations` all exist on `useTaskDetail()` — no context changes anywhere in this set.

## Phase 1: ExecutionPanel (REQ-001)
- [x] Move `parseLiveAction` out of `CurrentImplementationCard.tsx` into a shared util (e.g. `components/liveAction.ts`) so it survives that component's deletion; export unchanged.
- [x] Create `ExecutionPanel.tsx`: single card, **no inner bordered cards** — Row 1 = inline progress strip (progress bar + `%` + done/total + elapsed time from `workflow.checkpoints[0].created_at` ticker); Row 2 = flat single-column checklist (not `grid-cols-2`).
- [x] Checklist rows: done = check + muted strikethrough; pending = hollow circle; current = left accent border + pulse + inline live-action badge (`parseLiveAction(logs)` → "Editing: cron.go"), updating as logs arrive.
- [x] Row click calls the `onOpenLog(stepId)` callback prop (wired in Phase 5) — do not call `getElementById` directly from the panel.
- [x] Empty state (`implementationItems.length === 0`): minimal strip with status badge + current workflow step (`formatStepName`) + elapsed time; no checklist section heading.
- [x] File-count chip per item (`affectedFiles.length`) carried over from `ImplementationChecklist`.

## Phase 2: CompactTimeline (REQ-002)
- [x] Create `CompactTimeline.tsx`: vertical list — per row: 8px status dot → step name → flex-grow dotted leader → duration (`stepDurations`) → status icon (✓ / spinner / ✗). No connector lines, no 52px nodes, no hover tooltip cards, no sub-task trees.
- [x] Group all `code_*` per-unit steps into one "Implementation" row (per-unit detail is `ExecutionPanel`'s job); its status = running if any unit runs, done if all done.
- [x] Running row gets the pulsing indicator; card elevation `shadow-sm` (secondary tier); target ≤ 300px height for 8 steps.
- [x] Reuse `getSemanticStatusColor` for dot/icon colors — no new color mapping.
- [x] Styling (decided): no `backdrop-blur-xl`/gradient-hover on CompactTimeline or accordion rows — plain `bg-card` + `shadow-sm`; the glass treatment stays only on `ExecutionPanel`. Existing tokens/utilities only, no new CSS classes or custom properties.

## Phase 3: SupportingAccordion (REQ-003)
- [x] Create `SupportingAccordion.tsx` with a small `AccordionItem` (header = title + summary line + chevron; independent open state per item, **not** mutually exclusive) driven by an `openSections: Record<string, boolean>` map passed from `page.tsx`.
- [x] **Specification** item: mounts `SpecPanel` with `isExpanded={true}` + hidden/no-op outer toggle when the accordion is open; summary line = presence chips (Scope/Recommendation/Architecture/Risks) — lift the chip derivation out of `SpecPanel`'s collapsed header or recompute from `analysisData`.
- [x] **Execution Logs** item: keep `LogConsole` **mounted even when collapsed** (hidden via CSS) so `log-group-{stepId}` anchors resolve and the stream keeps buffering — pass `isExpanded` from accordion state; summary line = latest event (reuse the exported `parseMilestones` from `log-console.tsx`) with semantic coloring + warning indicator when the latest relevant event is an error.
- [x] **Description** item: extract the description body + edit-in-place UX + Request-history + Clarification-history blocks from `TaskHeader.tsx` (~lines 205–299) into a `DescriptionBody.tsx`; summary line = first ~80 chars of `descriptionParts.body` (or "No description").
- [x] **Agent & Checkpoints** item: create a small `CheckpointsPanel` (new — nothing renders `workflow.checkpoints` as a list today) showing agent name, attempts, last error, and the reversed checkpoint list with step name + timestamp; summary line = "N checkpoints".

## Phase 4: TaskHeader slimmed (REQ-004)
- [x] Remove the status/metrics strip (Current Step, Progress, Assigned Agent, Attempts, Last Error chips) — all now in `ExecutionPanel`.
- [x] Remove the description toggle, body, editor, and history blocks (moved to `DescriptionBody` in Phase 3).
- [x] Keep: back link, editable title, status badges (task/spec/job/priority), run controls (Analyze/Retry, Execute/Retry, Resume, Pause, Cancel, Delete + `ConfirmDialog`).
- [x] Confirm result is single-purpose and ~150 LOC.

## Phase 5: Page recomposition + deletions (REQ-M01, REQ-M02, REQ-R01–R05)
- [x] Rewrite `page.tsx` order: `TaskHeader` → `ReviewActionBar` → `ExecutionPanel` → `CompactTimeline` → `PRPanel` (conditional) → error/paused banners + `BoundaryResolutionControls` (unchanged blocks) → `SupportingAccordion` → `RequestChangesModal`.
- [x] Replace `[isLogExpanded, isSpecExpanded]` with the `openSections` accordion map; `expandAndScrollToLog(stepId)` = set `logs: true` then `requestAnimationFrame` scroll to `log-group-{stepId}` (REQ-M01).
- [x] State (decided): `openSections` is in-memory `useState` — no localStorage (REQ-003 mandates default-collapsed on load); `SpecPanel`'s inner `task-*-collapsed-${taskID}` localStorage keys stay untouched.
- [x] Delete `DashboardSummary.tsx`, `CurrentImplementationCard.tsx`, `ImplementationChecklist.tsx`, `WorkflowTimeline.tsx` (`git rm`); verify no remaining importers first (`grep`), then `npx tsc --noEmit` clean.
- [x] Verify `PRPanel` renders between timeline and accordion with unchanged internals (REQ-M02).

## Phase 6: E2E + verification
- [x] Rewrite `e2e/task-detail.spec.ts` test 1 for the new composition (its two sticky-bar tests survive unchanged; `e2e/mapping.spec.ts` survives — pure `deriveImplementationItems` logic): ExecutionPanel shows progress strip + highlighted current item; empty pre-analysis state; CompactTimeline is vertical (no `Workflow Phases` horizontal rail assertions); all four accordion rows start collapsed with summary lines; expanding one leaves others untouched (independence); checklist click expands Logs + scrolls; removed components (`DashboardSummary` metrics grid, `CurrentImplementationCard`, standalone checklist heading) absent from DOM; ReviewActionBar assertions carried over unchanged.
- [x] Reuse the existing `api-mocks.ts` workflow fixture (2 units → 1 running/1 pending) — extend only if an assertion needs a `done` item.
- [x] Run: `npx tsc --noEmit`, `npx eslint` on touched files, `npx playwright test` (note: with a dev server already running, use `SKIP_WEBSERVER=1 PLAYWRIGHT_PORT=<port>`); full suite green, no regressions in other specs.
- [x] Manual 3-second pass: what's running / done / remaining / waiting-for-me answerable from ExecutionPanel + ReviewActionBar without scrolling or expanding anything.
- [x] Update `specs.md` status icons `❌`→`✅` per requirement as it lands.

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
