# Expected Behavior: Update Feature Docs

## Scenario: Document CLI Execution Engine Hardening
**When:**
- A developer reads `product/14-execution-engine.md`
**Then:**
- It should explain that sandbox CLI credentials are now staged in a per-run auth directory.
- It should explain how context files are injected into CLI runtime environments.
- It should detail how CLI artifacts are versioned by attempt.
- It should explain the credential re-authentication status.

## Scenario: Document WebSocket Terminal
**When:**
- A developer reads `product/11-multi-channel-interaction.md`
**Then:**
- It should detail the WebSocket ticket-based authentication flow for secure CLI and remote terminal access.
- It should explain the PTY terminal resizing mechanism and WS reconnect resilience.
- It should mention how verified auth claims are consulted for RBAC.
- It should mention the `minttoken` tool for generating test tokens.

## Rules
- All docs must follow the existing conventions (Vietnamese body prose, English headers/identifiers).
- Each updated document's frontmatter `verified` date must be bumped to the current date (`2026-07-30`).

## Constraints
- Cross-references must use the full folder path.
