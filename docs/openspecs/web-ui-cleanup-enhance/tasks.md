# Tasks: Web UI Cleanup & Enhancement

## Phase 0 — Dead code removal (blocks everything else)

### Task 0.1: Extract CLI form into `execution-providers-list.tsx` & delete `cli-engine-config-form.tsx`
- [x] Move `CLIEngineConfigForm` component, `CLIEngineConfigFormValue` type, `cliConfigToFormValue`, `formValueToCLIConfig`, `formValuesEqual` into `execution-providers-list.tsx` (or a co-located `cli-engine-types.ts`).
- [x] Update imports in `execution-providers-list.tsx` to reference the local definitions.
- [x] Delete `web/src/components/projects/cli-engine-config-form.tsx`.
- [x] Verify: `next build` passes; Custom CLI row in Project Settings still expands and renders the inline form.
- Satisfies: REQ-M01

### Task 0.2: Remove `ExecutionEngine` / `cli_engine_config` from frontend types
- [x] Remove `ExecutionEngine` type alias from `web/src/lib/types.ts`.
- [x] Remove `CLIEngineConfig` type from `types.ts` (still needed by `execution-providers-list.tsx` — verify; if yes, keep it or move into `execution-providers-list.tsx`).
- [x] Remove `execution_engine?: ExecutionEngine` and `cli_engine_config?: CLIEngineConfig` from `Project` type.
- [x] Remove `execution_engine?: ExecutionEngine | null` from `Task` type.
- [x] Fix any resulting TypeScript errors across the codebase.
- Satisfies: REQ-R01

### Task 0.3: Clean up API layer + component references
- [x] Remove `execution_engine` and `cli_engine_config` from `UpdateProjectInput` in `web/src/lib/api/projects.ts`.
- [x] Remove `execution_engine` parameter from `createTask()` function signature and call body in `web/src/lib/api/projects.ts`.
- [x] Remove `execution_engine` from the payload construction in `web/src/lib/hooks/project/use-task-actions.ts`.
- [x] Remove residual `ExecutionEngine` import from `project-profile.tsx` if still present.
- [x] Remove `execution_engine`/`cli_engine_config` references from `app/projects/[id]/tasks/[taskID]/components/TaskTitleBlock.tsx` (found during re-audit, missing from original scope).
- [x] Remove `execution_engine`/`cli_engine_config` references from `app/projects/[id]/tasks/[taskID]/components/CLISpecPanel.tsx` (found during re-audit, missing from original scope).
- [x] Verify: `next build` passes.
- Satisfies: REQ-R01

### Task 0.4: Remove "Execution Engine" dropdown from Create Task modal
- [x] Remove `ExecutionEngine` import from `create-task-panel.tsx`.
- [x] Remove `cliConfigured` computation (lines referencing `project?.cli_engine_config?.command`).
- [x] Remove `executionEngine` state variable and its reset in `handleSubmit`.
- [x] Remove the "Execution Engine" `<Field>` + `<select>` JSX block.
- [x] Remove `execution_engine` from `CreateTaskPayload` type.
- [x] Verify: `next build` passes; Create Task modal renders without the dropdown.
- Satisfies: REQ-R03

### Task 0.5: Delete duplicate Field component
- [x] Delete `web/src/components/projects/Field.tsx`.
- [x] Update `create-task-panel.tsx` import from `"./Field"` → `"@/components/ui/field"`.
- [x] Verify: `next build` passes; Create Task modal field labels still render correctly.
- Satisfies: REQ-R02

---

## Phase 1 — Full design-token audit (reference list, consumed by Phase 2)

Repo-wide `grep -rc "bg-slate-\|text-slate-" web/src --include="*.tsx"` result — **43 files**, grouped by the page/section that owns them (fixed once, in that page's Phase 2 task, per the "fix once, earliest page" rule in `design.md`):

| Group | Files (occurrence count) |
|---|---|
| UI primitives | `ui/dialog.tsx` (1), `ui/confirm-dialog.tsx` (1), `ui/markdown.tsx` (2), `ui/switch.tsx` (1) |
| Settings | `settings/members/AgentCard.tsx` (2) |
| Rules | `app/rules/page.tsx` (1), `app/rules/components/AddRuleModal.tsx` (2), `app/rules/components/RuleList.tsx` (1), `app/rules/components/RuleListItem.tsx` (1) |
| Audit | `app/audit/page.tsx` (6) |
| Analytics | `app/analytics/page.tsx` (1), `app/analytics/components/AgentPerformanceTable.tsx` (6), `app/analytics/components/ProviderAnalytics.tsx` (1), `app/analytics/components/RecentFailures.tsx` (1), `app/analytics/components/VirtualKeyAnalytics.tsx` (1) |
| Knowledge (+ suggestions) | `app/knowledge/page.tsx` (1), `app/knowledge/components/AgentConfigSidebar.tsx` (3), `app/knowledge/components/MemoryCard.tsx` (5), `app/knowledge/components/MemoryInspector.tsx` (7), `app/knowledge/components/SearchBar.tsx` (3), `app/knowledge/suggestions/page.tsx` (4), `app/knowledge/suggestions/components/SuggestionCard.tsx` (5) |
| Projects (shared components) | `components/projects/AgentSelection.tsx` (2), `components/projects/create-task-panel.tsx` (1), `components/projects/GovernanceConfigEditor.tsx` (1), `components/projects/LearnedSkillsPanel.tsx` (8), `components/projects/task-clarification-form.tsx` (1), `components/projects/task-diff-viewer.tsx` (6), `components/projects/tasks-tab.tsx` (3), `app/projects/[id]/error.tsx` (2) |
| Task Detail | `app/projects/[id]/tasks/[taskID]/page.tsx` (1), `.../components/AuditPanel.tsx` (9), `.../components/BoundaryResolutionControls.tsx` (1), `.../components/CheckpointsPanel.tsx` (6), `.../components/CLISpecReviewControls.tsx` (1), `.../components/DescriptionBody.tsx` (1), `.../components/RequestChangesModal.tsx` (2), `.../components/ReviewActionBar.tsx` (1), `.../components/ReviewVerdictCard.tsx` (1), `.../components/SpecPanel.tsx` (1), `.../components/SupportingAccordion.tsx` (4), `.../components/TaskDetailContext.tsx` (2), `.../components/TaskDetailLayout.tsx` (1), `.../components/TaskHeader.tsx` (2), `.../components/TaskHeroCards.tsx` (9), `.../components/TaskSidebar.tsx` (2), `.../components/TaskSubtasks.tsx` (2), `.../components/TaskTitleBlock.tsx` (5) |
| Tasks list | `app/tasks/[id]/page.tsx` (2) |

Token mapping (unchanged from original design): `bg-slate-950`→`bg-background`, `bg-slate-950/80`→`bg-background/80`, `text-slate-200`→`text-foreground`, `text-slate-300`→`text-foreground/80` or `text-content-muted`, `text-slate-400`→`text-content-muted`, `text-slate-500`→`text-content-muted/70`, `text-slate-950`→`text-background`, `border-slate-500/20`→`border-stroke`.

---

## Phase 2 — Per-page cleanup & enhance loop

Work **one page at a time, in this order** (small/foundational first, largest/highest-risk last). Each task = design tokens (per Phase 1 list, if any) + dead-code audit + accessibility baseline + light visual polish (spacing/consistency only, no redesign — see REQ-E03), verified and reviewed before starting the next.

- [x] **2.1 UI primitives** (`components/ui/*`) — fix 4 files from Phase 1; dead-code/a11y/polish pass on the primitives every other page depends on. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.2 Settings** (`app/settings/page.tsx`, `components/settings/**`) — fix `AgentCard.tsx`; no other known color issues. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.3 Rules** (`app/rules/**`) — fix 4 files from Phase 1. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.4 Audit** (`app/audit/page.tsx`) — fix 6 occurrences in one file. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.5 Analytics** (`app/analytics/**`) — fix 5 files from Phase 1. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.6 Knowledge + Suggestions** (`app/knowledge/**`) — fix 7 files from Phase 1, largest non-project group. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.7 Agents** (`app/agents/page.tsx`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.8 AI Providers** (`app/ai-providers/page.tsx`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.9 Gateway** (`app/gateway/page.tsx`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.10 Git Accounts** (`app/git-accounts/page.tsx`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.11 Organization** (`app/organization/page.tsx`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.12 Skills** (`app/skills/page.tsx`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.13 Dashboard** (`app/page.tsx`, `components/dashboard/**`) — no known color issues; dead-code/a11y/polish only. Satisfies: REQ-E01, REQ-E02, REQ-E03
- [x] **2.14 Tasks list** (`app/tasks/[id]/page.tsx`) — fix 1 file (2 occurrences). Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.15 Projects (list/detail shell)** (`app/projects/[id]/page.tsx`, `error.tsx`, `components/projects/*` not already covered by Phase 0) — fix 8 files from Phase 1, includes the shared `projects/` components. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03
- [x] **2.16 Task Detail** (`app/projects/[id]/tasks/[taskID]/**`) — largest group, fix 18 files from Phase 1. Done last since it's highest risk/largest diff and depends on Phase 0's `TaskTitleBlock.tsx`/`CLISpecPanel.tsx` cleanup already landing. Satisfies: REQ-M02, REQ-E01, REQ-E02, REQ-E03

---

## Phase 3 — Verification

### Task 3.1: Full build verification
- [ ] Run `cd web && npx next build` — must complete with 0 errors.
- [ ] Run `cd web && npx next lint` — no unused-var/import warnings in touched files.
- [ ] Run `grep -r "bg-slate-\|text-slate-" web/src/ --include="*.tsx"` — must return 0 matches.
- [ ] Run `grep -r "execution_engine\|cli_engine_config" web/src/ --include="*.tsx" --include="*.ts"` — must return 0 matches (excluding this spec and comments).
- [ ] Verify `web/src/components/projects/Field.tsx` does not exist.
- [ ] Verify `web/src/components/projects/cli-engine-config-form.tsx` does not exist.
- Satisfies: REQ-R01, REQ-R02, REQ-R03, REQ-M01, REQ-M02, REQ-E01, REQ-E02, REQ-E03

## Self-review checklist

| REQ | Task |
|-----|------|
| REQ-R01 | 0.2, 0.3 |
| REQ-R02 | 0.5 |
| REQ-R03 | 0.4 |
| REQ-M01 | 0.1 |
| REQ-M02 | 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.14, 2.15, 2.16 |
| REQ-E01 | 2.1–2.16 (all) |
| REQ-E02 | 2.1–2.16 (all) |
| REQ-E03 | 2.1–2.16 (all) |
| All | 3.1 (verification) |
