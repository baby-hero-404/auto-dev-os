package handler

import (
	"log"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// wsTerminalTicketStore wraps a wsTicketStore with the org-check + mint +
// respond and ticket-consume + WS-upgrade + workspace glue shared by every
// ticket-gated terminal endpoint (CLIAuthHandler, CLITestHandler). Each
// handler still owns its own instance (auth tickets and test tickets are
// deliberately separate pools), and still implements its own pre/post step
// around this shared flow (the auth-capture banner + file walk vs. the
// credential-priming logic).
type wsTerminalTicketStore struct {
	tickets *wsTicketStore
}

func newWSTerminalTicketStore() *wsTerminalTicketStore {
	return &wsTerminalTicketStore{tickets: newWSTicketStore()}
}

// Mint mints a raw ticket, bypassing the HTTP glue below. Exists for tests
// that need a valid ticket without going through a full HTTP round trip.
func (s *wsTerminalTicketStore) Mint(userID, orgID, value string) (string, error) {
	return s.tickets.Mint(userID, orgID, value)
}

// mintTicket implements the shared POST .../ws-ticket handler body: org
// check, decode the caller-specific request body via decode (which returns
// the opaque value to bind the ticket to — a provider name or a credential
// ID — and ok=false on a bad/missing body), mint, then hand the token to
// respond to shape the JSON response (the two callers' response shapes
// differ: CLIAuthHandler includes expires_in, CLITestHandler doesn't).
func (s *wsTerminalTicketStore) mintTicket(
	w http.ResponseWriter,
	r *http.Request,
	decode func(r *http.Request) (value string, ok bool),
	badRequestMsg string,
	respond func(w http.ResponseWriter, ticket string),
) {
	orgID := chi.URLParam(r, "orgID")
	claims, ok := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if !ok || claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "org mismatch")
		return
	}

	value, ok := decode(r)
	if !ok {
		writeError(w, http.StatusBadRequest, badRequestMsg)
		return
	}

	ticket, err := s.tickets.Mint(claims.Subject, claims.OrgID, value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint ticket")
		return
	}
	respond(w, ticket)
}

// beginTerminal implements the shared GET .../terminal websocket handler
// prefix: ticket consume, WS upgrade, and per-session temp workspace
// creation. Returns ok=false once it has already written the HTTP/WS error
// response itself — the caller should just return in that case. On success
// the caller owns conn and must `defer conn.Close()` and `defer cleanup()`.
func (s *wsTerminalTicketStore) beginTerminal(w http.ResponseWriter, r *http.Request, workspacePrefix string) (conn *websocket.Conn, ticket wsTicket, taskID, tmpDir string, cleanup func(), ok bool) {
	orgID := chi.URLParam(r, "orgID")
	ticketStr := r.URL.Query().Get("ticket")
	if ticketStr == "" {
		http.Error(w, "missing ticket", http.StatusUnauthorized)
		return nil, wsTicket{}, "", "", nil, false
	}
	t, found := s.tickets.Consume(ticketStr, orgID)
	if !found {
		http.Error(w, "invalid, expired, or already-used ticket", http.StatusUnauthorized)
		return nil, wsTicket{}, "", "", nil, false
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket: %v", err)
		return nil, wsTicket{}, "", "", nil, false
	}

	tID, dir, clean, err := newTerminalWorkspace(workspacePrefix)
	if err != nil {
		sendWSError(c, "failed to create temp workspace: %v", err)
		c.Close()
		return nil, wsTicket{}, "", "", nil, false
	}

	return c, t, tID, dir, clean, true
}
