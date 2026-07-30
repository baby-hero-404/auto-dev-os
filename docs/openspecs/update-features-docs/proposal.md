# Proposal: Update Feature Docs to Match Recent Code

## Problem
Recent codebase developments—such as WebSocket ticket-based authentication, PTY terminal resizing, per-run auth directory staging for CLI agents, and CLI artifact versioning—have been implemented in code but are not yet reflected in the `docs/features/` specification documents.

## Goal
Update the `docs/features` directory to accurately reflect the current state of the codebase, ensuring new capabilities are properly documented and status badges reflect reality.

## Success
The `docs/features/` directory accurately describes the newly added WebSocket terminal capabilities and the hardened CLI execution engine.

## Decisions
- Update `product/14-execution-engine.md` to include per-run auth directory staging, context file injection, credential re-authentication status, and CLI artifact versioning by attempt.
- Reactivate `product/11-multi-channel-interaction.md` (change status from Deferred to In Progress/Implemented) to document the WebSocket ticket-based authentication, PTY resizing, and terminal reconnect logic.
- Add documentation for the `minttoken` development CLI tool used to generate test JWT tokens for authentication.

## Trade-offs
- Modifying existing feature docs keeps the documentation centralized but increases the density of `14-execution-engine.md`.

## Out of Scope
- No code logic changes to the Orchestrator, CLI Engine, or Auth handlers.

## Impact
- `docs/features/product/14-execution-engine.md`
- `docs/features/product/11-multi-channel-interaction.md`
- `docs/features/README.md` (freshness table)
