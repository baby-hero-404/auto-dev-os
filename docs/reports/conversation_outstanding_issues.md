# Conversation Outstanding Issues Report
Date: 2026-07-23

This document summarizes the outstanding issues, architectural discussions, and technical debt identified during the conversation regarding Sandbox Architecture and WebSocket Authentication.

## 1. Sandbox & CLI Integration Architecture
**Context**: We analyzed how reference projects (`ai-sdlc`, `multica`, `hermes-agent`) handle sandbox isolation and CLI integrations compared to Auto Code OS's current API-Native + Docker Fat Image approach.

**Outstanding Issue**:
Auto Code OS currently relies strictly on a heavy Docker container and custom API-native tool loops. To optimize API costs and allow users to leverage their own flat-rate CLI subscriptions (e.g., Claude Code, Cursor Agent), a Hybrid architecture is recommended but not yet implemented.

**Action Items**:
- [ ] **Design `SubagentSpawner` Interface**: Create an adapter pattern (similar to `ai-sdlc`) that allows tasks to optionally run via local subprocess CLIs instead of the default API-native tool loop.
- [ ] **Implement Git Worktree Isolation**: For subprocess CLI execution, implement a Git Worktree pool manager so tasks can run securely in isolated `.worktrees/<task-id>` directories without modifying the parent tree.
- [ ] **Environment Blocklist**: Implement environment variable sanitization (similar to `multica`'s blocklist) to ensure external CLIs do not break the host environment or leak context across tasks.

## 2. WebSocket Authentication Security (Token in URL)
**Context**: The interactive terminal (`cli-auth/terminal`) requires a full-duplex WebSocket connection. Because browser WebSocket APIs do not support setting custom `Authorization` headers, the JWT access token is currently passed via a URL query parameter (`?token=...`).

**Outstanding Issue**:
While HTTPS/WSS encrypts the packet over the network, placing tokens in the URL is a security risk because the URL (and token) is often logged in plain text by web servers (Nginx, Go Chi router) and saved in browser history.

**Action Items**:
- [ ] **Refactor WebSocket Authentication**: Migrate away from query parameter tokens to a more secure enterprise pattern. Two proposed solutions are open for consideration:
    - **Option A (Recommended - One-time Ticket)**: Create a REST endpoint (e.g., `POST /api/v1/auth/ws-ticket`) that requires standard Bearer auth and returns a short-lived (10-30s), single-use ticket. The frontend then connects via WebSocket using `?ticket=<short-lived-ticket>`.
    - **Option B (Sub-protocol)**: Pass the JWT token as a WebSocket sub-protocol from the frontend (`new WebSocket(url, ["access_token", "<token>"])`) and parse the `Sec-WebSocket-Protocol` header on the Go backend.
