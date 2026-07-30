# Specs: Web UI Cleanup & Enhancement

## Removed Requirements

### REQ-R01: Legacy Execution Engine UI surface
> ❌ Status: Not Started

The `ExecutionEngine` type, `execution_engine` and `cli_engine_config` fields on `Project` and `Task` types, and all frontend code referencing them are removed. Backend DB columns are untouched (backward compat). **Scope correction**: includes `TaskTitleBlock.tsx` and `CLISpecPanel.tsx`, found during re-audit and missing from the original file list.

**Scenario:**
- WHEN a developer searches `web/src/` for `execution_engine` or `cli_engine_config`
- THEN zero results are returned (excluding comments documenting the removal)
- AND `next build` completes without errors

### REQ-R02: Duplicate Field component
> ❌ Status: Not Started

`web/src/components/projects/Field.tsx` is deleted. All consumers use the canonical `web/src/components/ui/field.tsx`.

**Scenario:**
- WHEN `web/src/components/projects/Field.tsx` is checked
- THEN the file does not exist
- AND `create-task-panel.tsx` imports `Field` from `@/components/ui/field`
- AND `next build` passes

### REQ-R03: Task-level Execution Engine dropdown
> ❌ Status: Not Started

The "Execution Engine" dropdown in the Create Task modal is removed. Task creation no longer sends `execution_engine`.

**Scenario:**
- WHEN a user opens the Create Task modal
- THEN no "Execution Engine" field is visible
- AND submitting the form does NOT send `execution_engine` in the API payload

## Modified Requirements

### REQ-M01: `execution-providers-list.tsx` self-contained
> ❌ Status: Not Started

After `cli-engine-config-form.tsx` is deleted, `execution-providers-list.tsx` must still compile and render the "Custom CLI" inline editor correctly.

**Scenario:**
- WHEN `execution-providers-list.tsx` is loaded in the browser
- THEN the "Custom CLI" row still expands to show command/args/env/timeout fields
- AND no import errors exist for deleted modules
- AND `next build` passes

### REQ-M02: Design token consistency
> ❌ Status: Not Started

All `bg-slate-*` and `text-slate-*` classes in `web/src/` are replaced with semantic design tokens from the project's CSS variable system. **Scope correction**: the original requirement was verified against only 9 files; a repo-wide grep found 43. This requirement now covers the full list.

**Scenario:**
- WHEN `grep -r "bg-slate-\|text-slate-" web/src/ --include="*.tsx"` is run
- THEN zero matches are returned
- AND visual appearance in dark mode remains consistent (no broken contrast)
- AND `next build` passes

## New Requirements (Issue 5 — per-page loop)

### REQ-E01: Per-page dead code audit
> ❌ Status: Not Started

Each of the 16 top-level pages/sections under `web/src/app/` is checked for unused imports, props, variables, and dead conditional branches within the components it owns.

**Scenario:**
- WHEN `next lint` is run after a page's pass
- THEN no unused-variable/import warnings remain for files touched in that page
- AND no dead code is reintroduced by unrelated pages (no cross-page refactors bundled into one page's task)

### REQ-E02: Per-page accessibility baseline
> ❌ Status: Not Started

Each page's icon-only interactive elements have `aria-label`, images have `alt`, form inputs have associated labels, and focus-visible states are present on interactive elements.

**Scenario:**
- WHEN a page is checked with axe DevTools or manual keyboard navigation
- THEN no missing-label/missing-alt violations are reported for elements touched in that page's pass
- AND tab order reaches all interactive elements

### REQ-E03: Per-page visual/UX polish (scoped)
> ❌ Status: Not Started

Minor spacing/consistency fixes are allowed per page; layout redesign and new features are explicitly out of scope for this loop.

**Scenario:**
- WHEN a page's diff is reviewed
- THEN changes are limited to spacing, alignment, and token/consistency fixes
- AND no new components, routes, or behavioral changes are introduced
- AND the page is reviewed and approved before the next page's pass begins
