# Proposal: CLI Terminal WebSocket Ticket Auth

## Why

`docs/reports/conversation_outstanding_issues.md` §2 flagged that the interactive CLI auth terminal passes the JWT as a raw URL query parameter because browser `WebSocket` APIs cannot set custom `Authorization` headers.

Reading the actual (currently uncommitted) implementation confirms the risk is worse than the report assumed:

- `web/src/app/ai-providers/components/AddCredentialModal.tsx:117` builds `wsUrl` with the user's **full, long-lived access token** in the query string: `.../cli-auth/terminal?provider=...&token=${token}`.
- `server/internal/handler/auth.go:63-89` (`AuthMiddleware`) already accepts `?token=` as a fallback for WebSocket routes, so this URL is a valid, standing credential for as long as the access token is valid — not a short-lived artifact. It will be captured verbatim by any reverse proxy/access log (Nginx, Chi request logger) and persisted in browser history.
- `server/internal/handler/cli_auth.go`'s `Terminal` handler never reads the authenticated claims from context at all — it trusts the `{orgID}` path segment unconditionally. Combined with `AuthMiddleware` only checking that the token is *some* valid access token (not that its `OrgID` matches the path), **any authenticated user can open another org's CLI terminal** by substituting the org ID in the URL. This is a separate, more serious authorization gap discovered while investigating the transport issue.

Both problems are fixed by the same mechanism: a short-lived, single-use, org-bound ticket minted over a normal Bearer-authenticated REST call, exchanged for query-param use only once and only for ~20 seconds.

## What Changes

### Issue 1: Long-lived JWT exposed in WebSocket URL
- Add `POST /organizations/{orgID}/cli-auth/ws-ticket` (behind existing `AuthMiddleware` + `Bearer` header — never a query param) that mints a random opaque ticket bound to `(UserID, OrgID, Provider, ExpiresAt)`.
- `GET /organizations/{orgID}/cli-auth/terminal` now requires `?ticket=<t>` instead of `?token=<jwt>`. The raw JWT no longer appears in any URL for this feature.
- Ticket is single-use (deleted on first read) and short-lived (20s TTL) — a leaked URL is worthless after one use or 20 seconds, whichever comes first.

### Issue 2: Missing org-scope check in `Terminal` handler
- `Terminal` no longer trusts the `{orgID}` path param directly for authorization — it derives `OrgID`/`UserID`/`Provider` from the validated ticket record, so a ticket minted for org A can never open org B's terminal even if the URL is edited.

## Capabilities

### New Capabilities
- `POST /organizations/{orgID}/cli-auth/ws-ticket` REST endpoint (Bearer-authenticated, returns opaque ticket + short TTL).
- In-memory single-use ticket store (`internal/handler/ws_ticket_store.go`) with lazy expiry.

### Modified Capabilities
- `GET /organizations/{orgID}/cli-auth/terminal`: query param changes from `?token=<jwt>` to `?ticket=<opaque>`; authorization now derived from the ticket record, not the path.
- `AddCredentialModal.tsx`: calls the new ticket endpoint before opening the WebSocket, instead of embedding `token` directly.

### Removed Capabilities
- Raw JWT accepted via `?token=` for the `cli-auth/terminal` route specifically (the general `AuthMiddleware` query-param fallback for other WS routes, if any exist elsewhere, is out of scope for this change — see Design §Out of Scope).

## Impact

| Area | Files Affected |
|------|-----------------|
| Handler (new) | `server/internal/handler/ws_ticket_store.go` (new) |
| Handler (modified) | `server/internal/handler/cli_auth.go` |
| Router | `server/internal/handler/router.go` |
| Frontend | `web/src/app/ai-providers/components/AddCredentialModal.tsx` |
| Frontend API client | `web/src/lib/api/gateway.ts`, `web/src/lib/api/index.ts` |
