# Tasks: CLI Terminal WebSocket Ticket Auth

> **For agentic workers:** implement task-by-task; each step is checkbox-tracked.

**Goal:** Replace the raw-JWT-in-URL pattern on `cli-auth/terminal` with a short-lived, single-use, org-bound ticket, and close the org-scope authorization gap in the same change.

**Architecture:** New `POST .../ws-ticket` (Bearer-authenticated) mints an opaque 20s single-use ticket via a new in-memory `wsTicketStore`; `Terminal`'s WS handshake consumes the ticket instead of reading `?token=`. Full sequence in `design.md`.

**Tech Stack:** Go (chi router, gorilla/websocket), TypeScript/React (Next.js), existing `AuthMiddleware`/`AuthService`.

---

## P0 — Critical

### Task 1.1: Ticket store (mint + single-use consume + TTL)
> Links to: REQ-001, REQ-003, REQ-004

**Files:**
- Create: `server/internal/handler/ws_ticket_store.go`
- Test: `server/internal/handler/ws_ticket_store_test.go`

- [x] **Step 1: Write the failing tests**

```go
package handler

import (
	"testing"
	"time"
)

func TestWSTicketStore_MintAndConsume(t *testing.T) {
	s := newWSTicketStore()
	token, err := s.Mint("user-1", "org-1", "claude")
	if err != nil {
		t.Fatalf("mint failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty ticket")
	}

	got, ok := s.Consume(token, "org-1")
	if !ok {
		t.Fatal("expected ticket to be valid")
	}
	if got.UserID != "user-1" || got.OrgID != "org-1" || got.Provider != "claude" {
		t.Errorf("unexpected ticket data: %+v", got)
	}
}

func TestWSTicketStore_SingleUse(t *testing.T) {
	s := newWSTicketStore()
	token, _ := s.Mint("user-1", "org-1", "claude")

	if _, ok := s.Consume(token, "org-1"); !ok {
		t.Fatal("first consume should succeed")
	}
	if _, ok := s.Consume(token, "org-1"); ok {
		t.Fatal("second consume of same ticket must fail")
	}
}

func TestWSTicketStore_Expiry(t *testing.T) {
	s := newWSTicketStore()
	token, _ := s.Mint("user-1", "org-1", "claude")

	// Force expiry by rewriting the stored entry's ExpiresAt into the past.
	s.mu.Lock()
	entry := s.tickets[token]
	entry.ExpiresAt = time.Now().Add(-1 * time.Second)
	s.tickets[token] = entry
	s.mu.Unlock()

	if _, ok := s.Consume(token, "org-1"); ok {
		t.Fatal("expired ticket must not be consumable")
	}
}

func TestWSTicketStore_OrgMismatch(t *testing.T) {
	s := newWSTicketStore()
	token, _ := s.Mint("user-1", "org-A", "claude")

	if _, ok := s.Consume(token, "org-B"); ok {
		t.Fatal("ticket minted for org-A must not validate against org-B")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/handler/... -run TestWSTicketStore -v`
Expected: FAIL with `undefined: newWSTicketStore` (file doesn't exist yet)

- [x] **Step 3: Write minimal implementation**

Create `server/internal/handler/ws_ticket_store.go` with the full `wsTicket`, `wsTicketStore`, `newWSTicketStore`, `Mint`, `Consume`, `evictExpiredLocked` code exactly as specified in `design.md` → Data Models (first code block).

- [x] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/handler/... -run TestWSTicketStore -v`
Expected: PASS (all 4 subtests)

- [x] **Step 5: Commit**

```bash
git add server/internal/handler/ws_ticket_store.go server/internal/handler/ws_ticket_store_test.go
git commit -m "feat(cli-auth): add single-use short-lived WS ticket store (REQ-001,003,004)"
```

### Task 1.2: `MintWSTicket` handler + route
> Links to: REQ-001, REQ-M01, REQ-006

**Files:**
- Modify: `server/internal/handler/cli_auth.go` (add `tickets` field + `MintWSTicket` method; see design.md second code block)
- Modify: `server/internal/handler/router.go:148` (add sibling route)
- Test: `server/internal/handler/cli_auth_test.go`

- [x] **Step 1: Write the failing test**

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
)

func TestMintWSTicket_Success(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	claims := &service.TokenClaims{Subject: "user-1", OrgID: "org-1"}
	req := httptest.NewRequest("POST", "/organizations/org-1/cli-auth/ws-ticket", strings.NewReader(`{"provider":"claude"}`))
	req = req.WithContext(context.WithValue(req.Context(), authClaimsKey, claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.MintWSTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad response body: %v", err)
	}
	if resp.Ticket == "" || resp.ExpiresIn != 20 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestMintWSTicket_OrgMismatchForbidden(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	claims := &service.TokenClaims{Subject: "user-1", OrgID: "org-1"}
	req := httptest.NewRequest("POST", "/organizations/org-2/cli-auth/ws-ticket", strings.NewReader(`{"provider":"claude"}`))
	req = req.WithContext(context.WithValue(req.Context(), authClaimsKey, claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-2")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.MintWSTicket(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/handler/... -run TestMintWSTicket -v`
Expected: FAIL with `h.MintWSTicket undefined` / `NewCLIAuthHandler(nil)` signature mismatch (ticket store field doesn't exist yet)

- [x] **Step 3: Write minimal implementation**

Apply the `cli_auth.go` additions from `design.md` (the `CLIAuthHandler` struct gains `tickets *wsTicketStore`, `NewCLIAuthHandler` initializes it, add `MintWSTicket`). Add import for `"github.com/go-chi/chi/v5"` and `service` package. Register the route in `router.go` right after line 148:

```go
r.Get("/cli-auth/terminal", cliAuthH.Terminal)
r.Post("/cli-auth/ws-ticket", cliAuthH.MintWSTicket)
```

- [x] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/handler/... -run TestMintWSTicket -v`
Expected: PASS (both subtests)

- [x] **Step 5: Commit**

```bash
git add server/internal/handler/cli_auth.go server/internal/handler/cli_auth_test.go server/internal/handler/router.go
git commit -m "feat(cli-auth): add POST ws-ticket endpoint, Bearer-authenticated (REQ-001,M01,006)"
```

### Task 1.3: `Terminal` consumes ticket instead of raw token/provider query param
> Links to: REQ-002, REQ-003, REQ-004, REQ-006, REQ-007, REQ-R01

**Files:**
- Modify: `server/internal/handler/cli_auth.go:71-77` (replace `provider := r.URL.Query().Get("provider")` block)
- Test: `server/internal/handler/cli_auth_test.go` (append)

- [x] **Step 1: Write the failing test**

```go
func TestTerminal_MissingTicket_Unauthorized(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	req := httptest.NewRequest("GET", "/organizations/org-1/cli-auth/terminal", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Terminal(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestTerminal_InvalidTicket_Unauthorized(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	req := httptest.NewRequest("GET", "/organizations/org-1/cli-auth/terminal?ticket=does-not-exist", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Terminal(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestTerminal_TicketFromDifferentOrg_Unauthorized(t *testing.T) {
	h := NewCLIAuthHandler(nil)
	token, _ := h.tickets.Mint("user-1", "org-A", "claude")

	req := httptest.NewRequest("GET", "/organizations/org-B/cli-auth/terminal?ticket="+token, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-B")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Terminal(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for cross-org ticket, got %d", rr.Code)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/handler/... -run TestTerminal -v`
Expected: FAIL — current `Terminal` reads `provider` from query and returns 400 "missing provider" (not 401), and never checks any ticket, so `TestTerminal_TicketFromDifferentOrg_Unauthorized` would incorrectly proceed to `upgrader.Upgrade` today

- [x] **Step 3: Write minimal implementation**

Replace the top of `Terminal` (`cli_auth.go:71-77`) with the ticket-consuming version from `design.md`'s third code block: read `orgID` via `chi.URLParam`, read+consume `ticket` query param, return 401 on missing/invalid/cross-org, derive `provider` from the consumed ticket record. Keep everything from `conn, err := upgrader.Upgrade(...)` onward unchanged.

- [x] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/handler/... -run TestTerminal -v`
Expected: PASS (all 3 subtests)

- [x] **Step 5: Commit**

```bash
git add server/internal/handler/cli_auth.go server/internal/handler/cli_auth_test.go
git commit -m "fix(cli-auth): Terminal consumes single-use org-bound ticket, drop raw token param (REQ-002,003,004,006,007,R01)"
```

## P1 — High

### Task 2.1: Frontend mints ticket before opening WebSocket
> Links to: REQ-005

**Files:**
- Modify: `web/src/lib/api/gateway.ts` (add `cliAuth.mintWSTicket`)
- Modify: `web/src/lib/api/index.ts` (export it on the `api` object)
- Modify: `web/src/app/ai-providers/components/AddCredentialModal.tsx:113-117` (mint ticket, build `wsUrl` with `?ticket=`)

- [x] **Step 1: Add the API client function**

```typescript
// web/src/lib/api/gateway.ts — append
export const cliAuth = {
  mintWSTicket: (orgID: string, token: string, provider: string) =>
    request<{ ticket: string; expires_in: number }>(
      `/organizations/${orgID}/cli-auth/ws-ticket`,
      { method: "POST", token, body: JSON.stringify({ provider }) }
    ),
};
```

```typescript
// web/src/lib/api/index.ts — append inside the exported `api` object
mintCliAuthWSTicket: gateway.cliAuth.mintWSTicket,
```

- [x] **Step 2: Replace direct token embedding with a minted ticket**

In `AddCredentialModal.tsx`, the block building `wsUrl` (currently `AddCredentialModal.tsx:113-117`) changes from a synchronous string template to an async mint-then-connect flow, matching the component's existing `async` handlers elsewhere in the file:

```tsx
const [wsUrl, setWsUrl] = useState("");

async function openTerminal() {
  if (!orgID || !token) return;
  const { ticket } = await api.mintCliAuthWSTicket(orgID, token, form.provider);
  const wsBaseUrl = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:32080/api/v1").replace(/^http/, "ws");
  setWsUrl(`${wsBaseUrl}/organizations/${orgID}/cli-auth/terminal?provider=${form.provider}&ticket=${ticket}`);
}
```

Call `openTerminal()` at the point the modal currently switches into the "show terminal" step (wherever `wsUrl` was previously computed inline before rendering `<InteractiveTerminal wsUrl={wsUrl} .../>`), and gate rendering `<InteractiveTerminal>` on `wsUrl` being non-empty.

- [x] **Step 3: Manual verification (no WS unit test harness in this repo — see Self-Review Checklist)**

Run: `cd web && npm run dev`
Steps: open AI Providers page → Add Credential → pick a `cli:` provider → confirm Network tab shows `POST .../cli-auth/ws-ticket` (Bearer header, no token in URL) immediately followed by a WS connection to `.../cli-auth/terminal?provider=...&ticket=...` (no `token=` param anywhere).
Expected: terminal opens and streams output exactly as before this change.

- [x] **Step 4: Commit**

```bash
git add web/src/lib/api/gateway.ts web/src/lib/api/index.ts web/src/app/ai-providers/components/AddCredentialModal.tsx
git commit -m "feat(ai-providers): mint WS ticket before opening CLI auth terminal, drop raw token from URL (REQ-005)"
```

## P2 — Medium
(none)

## P3 — Low
(none)

---

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — target: `product/01-unified-ai-gateway.md` (CLI credential auth flow section, if present) or `product/05-git-integration.md`; confirm exact target doc when this ships, per the existing 41-set "Docs sync" convention (see `docs/features/README.md:77`)

---

## Self-Review Checklist

1. **Spec coverage:** REQ-001→Task 1.1/1.2, REQ-002/003/004→Task 1.1/1.3, REQ-005→Task 2.1, REQ-006/007→Task 1.2/1.3, REQ-M01→Task 1.2, REQ-R01→Task 1.3. All 8 requirements covered.
2. **Placeholder scan:** none — all steps have concrete code/commands.
3. **Type consistency:** `wsTicket`/`wsTicketStore` names match between `design.md` and this file; `TokenClaims.Subject`/`OrgID` match `server/internal/service/auth.go:28-35`.
4. **File paths:** all verified against current repo state (`cli_auth.go`, `router.go:148`, `AddCredentialModal.tsx:113-117`, `gateway.ts`, `index.ts` all exist today).
5. **No WS integration test**: gorilla/websocket upgrade requires a real TCP listener (`httptest.NewServer`, not `httptest.NewRecorder`) to test end-to-end; Task 2.1 Step 3 is manual for that reason. If stricter coverage is wanted later, add a `httptest.NewServer`-based test dialing a real `ws://` URL — flagged here rather than silently skipped.
