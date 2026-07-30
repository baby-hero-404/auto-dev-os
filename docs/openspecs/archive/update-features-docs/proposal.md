# Proposal: Comprehensive Feature Docs Audit & Sync

## Problem
The `docs/features/` directory serves as the source of truth for Auto Code OS capabilities, but it easily drifts out of sync with the actual codebase. Incremental updates are insufficient because they miss deprecated features, architectural refactors, and implicitly added capabilities. We lack a systematic approach to create, update, or remove documentation based on the *actual* codebase reality.

## Goal
Establish and execute a comprehensive methodology to audit the entire `server/` and `web/` codebase, mapping actual implementations to the `docs/features/` index. Create, update, or remove documentation to ensure 100% parity with the current system state.

## Success
Every feature documented in `docs/features/` is verifiable in the codebase. Every major capability in the codebase (e.g., Gateway, Governance, Orchestrator, CLI Engine, Tooling, Auth) has an accurate, up-to-date specification document. Outdated docs are removed or marked as deprecated.

## Decisions
- Adopt a **Code-First Audit Methodology**: We will scan the entry points (`server/internal/*`, `web/src/*`) and trace the domain logic to identify what exists.
- Docs that describe removed systems (e.g., deprecated patch engines) will be deleted or moved to an `archive/` folder.
- Docs that describe heavily refactored systems (e.g., Orchestrator execution units, workflow DAGs) will be completely rewritten.
- New undocumented domains (e.g., `tool/`, `policy/`, `governance/`) will receive new `product/` or `engineering/` specs.

## Trade-offs
- A full audit takes significant time compared to surgical updates, but it prevents accumulating documentation debt and misleading AI agents that rely on these specs.

## Out of Scope
- No code refactoring. If we find bad code, we document it as-is or note the technical debt, but we do not fix the code in this task.

## Impact
- Entire `docs/features/` directory tree.
