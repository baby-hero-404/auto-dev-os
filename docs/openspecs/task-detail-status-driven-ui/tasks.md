# Tasks: Status-Driven Task Detail UI

## Phase 1: Planning (Completed)
- [x] Author OpenSpec documents (`proposal.md`, `specs.md`, `design.md`, `tasks.md`) to define the Status-Driven architectural approach.

## Phase 2: Breadcrumb & Title Consolidation
- [x] Revert the breadcrumb in `TaskHeader.tsx` to `← Back to Project` to resolve the missing project name issue and align with original UX.
- [x] Refactor Description placement: Move it directly under the Task Title in `TaskTitleBlock`, make it collapsible, and retain edit functionality (instead of putting it in the bottom accordion).

## Phase 3: Status-Driven Hero Cards
- [x] Review Hero Cards to ensure they properly highlight the most critical information per status.
- [x] Integrate `SpecPanel` component into `TaskHeroCards.tsx` for robust `spec_review` rendering (replacing raw markdown dumps).
- [x] Integrate `LogConsole` into `TaskHeroCards.tsx` for dynamic execution visualization during `coding`, `testing`, `fixing`, and `failed` states.

## Phase 4: Persistent Context Integration
- [x] Refactor `SupportingAccordion` to act strictly as a historical reference/fallback (Spec, Logs, Checkpoints), without taking over primary elements like the Description.
- [x] Conditionally hide the Specification and Execution Logs accordions when they are actively being displayed as Hero Cards to prevent UI duplication and strictly enforce the status-driven approach.
- [x] Ensure all states are properly handled and the UI feels cohesive and uncluttered.

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
