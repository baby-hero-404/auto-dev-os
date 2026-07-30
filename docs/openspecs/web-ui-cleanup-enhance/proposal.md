# Proposal: Web UI Cleanup & Enhancement

## Why

After several rounds of feature development (Execution Provider Routing, CLI Spec-First Flow, CLI Auth, etc.), the frontend codebase has accumulated:

1. **Dead code**: The legacy "Execution Engine" UI was removed from `project-profile.tsx`, but the underlying form component (`cli-engine-config-form.tsx`), legacy type fields (`ExecutionEngine`, `execution_engine`, `cli_engine_config` on `Project`/`Task`), and API payload references still remain scattered across `types.ts`, `api/projects.ts`, `create-task-panel.tsx`, and `use-task-actions.ts`.
2. **Duplicate component**: `web/src/components/projects/Field.tsx` is a simplified copy of the canonical `web/src/components/ui/field.tsx`. The duplicate is imported by `create-task-panel.tsx` — the only consumer — creating confusion about which `Field` to use.
3. **Hardcoded colors**: Multiple components use raw Tailwind color classes (`bg-slate-950`, `text-slate-300/400/500`) instead of the project's design token system (`bg-background`, `text-foreground`, `text-content-muted`). This breaks theme consistency and makes dark/light mode transitions impossible.
4. **Stale "Execution Engine" dropdown in Create Task**: The "Execution Engine" `<select>` in `create-task-panel.tsx` still offers a binary `api_native | cli` choice inherited from the old model. With the new Execution Providers system, this dropdown is misleading — the routing decision is now automatic per-project priority list.

### Issue 5: Full per-page cleanup & enhance loop
A prior audit of Issue 3 only checked 9 hand-picked files. A repo-wide grep found **43 files** with `bg-slate-*`/`text-slate-*` classes, spanning pages the original spec never looked at (`analytics/`, `knowledge/`, `rules/`, `audit/`, and 19 of the 24 files under the task-detail view). Rather than patch the file list again, this scope is generalized into a page-by-page loop covering **all 16 top-level pages/sections** in `web/src/app/`, so nothing is missed and future drift is caught systematically. For each page, the loop checks:
1. **Design tokens** — hardcoded `slate`/`gray` colors → semantic tokens (same mapping as Issue 3).
2. **Dead code** — unused imports, props, variables, dead conditionals (same spirit as Issue 1).
3. **Accessibility baseline** — icon-only buttons need `aria-label`, images need `alt`, form inputs need associated labels, focus-visible states present.
4. **Light visual/UX polish** — spacing and consistency only; no layout redesign, no new features.

Pages are worked **one at a time**, each verified (`next build` + `next lint` + manual browser check) and reviewed before moving to the next, to keep diffs reviewable. See `tasks.md` for the ordered list.

## What Changes

### Issue 1: Remove legacy Execution Engine dead code
- Delete `web/src/components/projects/cli-engine-config-form.tsx` (224 lines) — its only remaining consumer is `execution-providers-list.tsx`, which imports the `CLIEngineConfig` form for the "Custom CLI" row's inline editor. **Refactor**: extract the minimal types/converters that `execution-providers-list.tsx` actually needs into `execution-providers-list.tsx` itself or a small shared types file; delete the rest.
- Remove `ExecutionEngine` type alias and `execution_engine`, `cli_engine_config` fields from `Project` type in `types.ts`.
- Remove `execution_engine` field from `Task` type in `types.ts`.
- Remove `execution_engine`, `cli_engine_config` from `UpdateProjectInput` in `api/projects.ts`.
- Remove `execution_engine` from `createTask` API call in `api/projects.ts`.
- Clean up `use-task-actions.ts` references.
- **Correction (found during re-audit)**: two more files reference these fields and were missing from the original scope — `app/projects/[id]/tasks/[taskID]/components/TaskTitleBlock.tsx` and `.../CLISpecPanel.tsx`. Both are in scope now.

### Issue 2: Remove duplicate Field component
- Delete `web/src/components/projects/Field.tsx` (24 lines).
- Update `create-task-panel.tsx` to import `Field` from `@/components/ui/field` (already used elsewhere in the project).

### Issue 3: Replace hardcoded colors with design tokens (full codebase)
- **Superseded scope**: the original list of 9 files covered only a fraction of the problem. A repo-wide grep of `bg-slate-\|text-slate-` across `web/src` returns **43 files** (see `tasks.md` Phase 1 for the full grouped list, ~110 total occurrences).
- Use semantic tokens: `bg-background`, `bg-surface`, `text-foreground`, `text-content-muted`, `border-stroke` (see Design Token Replacement Map in `design.md`).

### Issue 4: Remove stale "Execution Engine" dropdown from Create Task modal
- Remove the "Execution Engine" `<Field>` + `<select>` block from `create-task-panel.tsx`.
- Remove `execution_engine` from `CreateTaskPayload` type.
- The routing is now handled automatically by the project's `execution_providers` priority list.

## Capabilities

### Removed Capabilities
- Legacy `execution_engine` / `cli_engine_config` fields no longer sent from frontend (backend fields remain in DB for backward compat, but UI no longer exposes them).
- Task-level "Execution Engine" override dropdown removed from Create Task modal.
- Duplicate `Field` component deleted.

### Modified Capabilities  
- `create-task-panel.tsx` simplified (no engine selector).
- `execution-providers-list.tsx` self-contained (no dependency on full `cli-engine-config-form.tsx`).
- All UI components use semantic design tokens instead of hardcoded slate colors, across the entire `web/src/app` page tree, not just the components touched by Issues 1-4.
- Each page/section additionally gets a dead-code, accessibility, and light visual-polish pass (Issue 5).

## Impact

| Area | Files Affected |
|------|----------------|
| Dead code removal | `cli-engine-config-form.tsx`, `types.ts`, `api/projects.ts`, `use-task-actions.ts`, `execution-providers-list.tsx`, `TaskTitleBlock.tsx`, `CLISpecPanel.tsx` |
| Duplicate Field | `projects/Field.tsx` (delete), `create-task-panel.tsx` (update import) |
| Hardcoded colors | 43 files across `ui/`, `settings/`, `projects/`, and `app/{analytics,audit,knowledge,rules,projects,tasks}/` — full list in `tasks.md` Phase 1 |
| Stale dropdown | `create-task-panel.tsx`, `CreateTaskPayload` type |
| Per-page loop | All 16 top-level pages/sections under `web/src/app/` — see `tasks.md` Phase 2 |
