# Specs: CLI Terminal WebSocket Ticket Auth

## Added Requirements

### REQ-001: Mint one-time ticket over Bearer-authenticated REST call
> ✅ Status: Done

**Scenario:**
- WHEN an authenticated user (valid `Authorization: Bearer <jwt>`) calls `POST /organizations/{orgID}/cli-auth/ws-ticket` with `{"provider": "claude"}`
- THEN the response is `200` with `{"ticket": "<opaque-random-string>", "expires_in": 20}`
- AND the ticket is stored server-side bound to `(UserID, OrgID, Provider, ExpiresAt = now+20s)`
- AND the ticket value contains no encoded JWT/PII (random UUID/bytes only)

### REQ-002: Ticket required to open terminal WebSocket
> ✅ Status: Done

**Scenario:**
- WHEN a client opens `GET /organizations/{orgID}/cli-auth/terminal?provider=claude&ticket=<t>` (WebSocket upgrade)
- AND `<t>` is a valid, unexpired, unused ticket minted for this `{orgID}`
- THEN the WebSocket upgrade succeeds and the terminal session starts as today

### REQ-003: Ticket is single-use
> ✅ Status: Done

**Scenario:**
- WHEN the same ticket `<t>` is used a second time to open `.../cli-auth/terminal?ticket=<t>`
- THEN the server rejects the upgrade (HTTP 401 before upgrade, or closes the socket immediately with an error frame if upgrade already occurred) with a message indicating the ticket was already consumed

### REQ-004: Ticket expires after TTL
> ✅ Status: Done

**Scenario:**
- WHEN a ticket `<t>` is minted at `T0` and a client attempts to open the terminal at `T0 + 25s` (TTL = 20s)
- THEN the server rejects the connection as expired, even though the ticket was never previously used

### REQ-005: No raw JWT in `cli-auth/terminal` URL
> ✅ Status: Done

**Scenario:**
- WHEN the frontend opens the CLI auth terminal WebSocket
- THEN the connection URL contains only `provider` and `ticket` query params
- AND no `token=` parameter carrying the JWT is present anywhere in the URL

### REQ-006: Ticket is org-scoped — cannot be replayed against a different org
> ✅ Status: Done

**Scenario:**
- WHEN a ticket `<t>` is minted via `POST /organizations/{orgA}/cli-auth/ws-ticket`
- AND a client attempts `GET /organizations/{orgB}/cli-auth/terminal?ticket=<t>` where `orgB != orgA`
- THEN the server rejects the connection (ticket's bound `OrgID` does not match path `{orgID}`), regardless of whether `<t>` is otherwise valid/unused

### REQ-007: `Terminal` handler derives identity from ticket, not path
> ✅ Status: Done

**Scenario:**
- WHEN the `Terminal` handler processes a valid ticket
- THEN `UserID`/`OrgID`/`Provider` used for the sandbox session and any audit logging come from the ticket record resolved server-side
- AND the `{orgID}` path segment is only used to confirm it matches the ticket's bound `OrgID` (REQ-006), never as a trust boundary on its own

## Modified Requirements

### REQ-M01: `POST /cli-auth/ws-ticket` requires standard Bearer auth
> ✅ Status: Done

**Scenario:**
- WHEN a request to `POST /organizations/{orgID}/cli-auth/ws-ticket` has no `Authorization: Bearer` header
- THEN the request is rejected `401` by the existing `AuthMiddleware`, before reaching the ticket-minting handler (no new auth bypass introduced)

## Removed Requirements
- REQ-R01: `GET /organizations/{orgID}/cli-auth/terminal` no longer accepts `?token=<jwt>` as a valid credential (replaced by `?ticket=`, REQ-002/REQ-005).
