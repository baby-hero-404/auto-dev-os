package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/baby-hero-404/aicli/capture"
	"github.com/baby-hero-404/aicli/capture/ticket"
	"github.com/baby-hero-404/aicli/capture/wsgorilla"
	"github.com/baby-hero-404/aicli/sandbox"
	"github.com/go-chi/chi/v5"
)

// terminal is the ticket-gated WebSocket terminal shared by the CLI login (CLIAuthHandler) and the
// credential test (CLITestHandler). aicli does the work: single-use tickets, the origin check, the
// sandboxed PTY, cancelling the session when the browser goes away, and collecting files without
// following symlinks. Each handler owns its own instance, so auth tickets and test tickets stay
// separate pools.
type terminal struct {
	tickets ticket.Store
	ws      *wsgorilla.Handler
}

// newTerminal returns a terminal whose sessions run on runtime in a workspace under
// <tmp>/workspacePrefix. Only the browser origins in allowedOrigins may open the socket; configure
// adds what is specific to the handler (banner, seeded files, ...).
func newTerminal(runtime sandbox.Runtime, workspacePrefix string, allowedOrigins []string, configure func(*wsgorilla.Handler)) *terminal {
	tickets := ticket.NewMemory(0)
	h := &wsgorilla.Handler{
		Tickets:        tickets,
		Session:        &capture.Session{Runtime: runtime, WorkspacePrefix: workspacePrefix},
		AllowedOrigins: allowedOrigins,
	}
	if configure != nil {
		configure(h)
	}
	return &terminal{tickets: tickets, ws: h}
}

// mintTicket implements the shared POST .../ws-ticket handler body: org check, decode the
// caller-specific request body via decode (which returns the opaque value to bind the ticket to — a
// provider name or a credential ID — and ok=false on a bad/missing body), mint, then hand the token
// to respond to shape the JSON response.
func (t *terminal) mintTicket(
	w http.ResponseWriter,
	r *http.Request,
	decode func(r *http.Request) (value string, ok bool),
	badRequestMsg string,
	respond func(w http.ResponseWriter, token string),
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
	token, err := t.tickets.Mint(r.Context(), ticket.Ticket{Subject: claims.Subject, Scope: claims.OrgID, Value: value})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint ticket")
		return
	}
	respond(w, token)
}

// serve upgrades the request to a terminal session. The ticket must have been minted for the
// organization in the path.
func (t *terminal) serve(w http.ResponseWriter, r *http.Request) {
	t.ws.ServeTerminal(w, r, chi.URLParam(r, "orgID"))
}
