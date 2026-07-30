# Design: Audit Methodology

This document outlines *how* we will execute the comprehensive codebase-to-docs audit.

## Phase 1: Codebase Discovery
We will map the major domains by analyzing `server/internal/`:
- `gateway`, `orchestrator`, `workflow`, `sandbox`, `prompts`, `tool`
- `policy`, `governance`, `gitops`, `repository`, `observability`
- `context`, `database`, `handler`, `middleware`, `service`

For each domain, we identify the core models, main interfaces, and execution flow.

## Phase 2: Gap Analysis
We will compare the discovered domains against the current `docs/features/` index:
- **Missing Docs:** What domains exist in code but have no documentation? (e.g., `tool/`, `policy/`, `governance/` might be missing or under-documented).
- **Outdated Docs:** What docs describe mechanisms that have been completely rewritten? (e.g., if Orchestrator moved to `tool-based` agents, old patch engine docs might be obsolete).
- **Orphaned Docs:** What docs describe features that no longer exist in the codebase?

## Phase 3: Execution
1. **Remove/Archive:** Delete orphaned documents and update the `README.md` index.
2. **Create:** Write new feature docs for missing domains using the standard template (Frontmatter `sources:`, `Status:`, `Mục tiêu:`, Vietnamese body).
3. **Update:** Rewrite outdated documents based on the new interfaces and implementations found in `Phase 1`.

## Phase 4: Validation
A final pass to ensure all `sources:` links in the documentation are valid and that `docs/features/README.md` freshness index is 100% accurate.
