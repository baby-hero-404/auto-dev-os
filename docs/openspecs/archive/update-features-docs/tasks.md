# Tasks: Feature Docs Audit & Sync

## Docs sync
- Updates the ENTIRE `docs/features/` directory based on the audit.

## Execution

### Phase 1: Codebase Discovery & Gap Analysis
- [x] List all core packages in `server/internal/` and determine their primary responsibilities.
- [x] Compare findings with `docs/features/product/`, `docs/features/engineering/`, and `docs/features/hardening/`.
- [x] Create an artifact listing (1) Docs to Remove, (2) Docs to Update, (3) Docs to Create.

### Phase 2: Cleanup
- [x] Remove or archive orphaned documentation files (None found).
- [x] Remove them from `docs/features/README.md` (None removed).

### Phase 3: Create & Update
- [x] Draft new documents for undocumented domains (e.g., `governance`, `policy`, `tool`).
- [x] Rewrite outdated documents to reflect the actual implementation (e.g., Gateway, Orchestrator, CLI Engine).
- [x] Ensure newly touched docs have accurate `sources:` frontmatter.
- [x] **Deferred/Outstanding:** Retrofit exact `sources:` frontmatter onto the ~12 legacy docs that currently use `["server/**"]` globs. (This is a large task scoped out of the current audit). *(Completed in a follow-up phase)*

### Phase 4: Finalize
- [x] Regenerate the `docs/features/README.md` index and freshness table.
