package handler

import (
	"fmt"
	"net/http"

	"github.com/baby-hero-404/aicli/capture/ticket"
	"github.com/baby-hero-404/aicli/capture/wsgorilla"
	"github.com/baby-hero-404/aicli/sandbox"
)

// CLIAuthHandler runs an interactive login of a CLI in a sandbox and hands the files it wrote back
// to the browser, which stores them as a provider credential.
type CLIAuthHandler struct {
	term *terminal
}

// NewCLIAuthHandler returns the handler. Only the browser origins in allowedOrigins may open the
// terminal socket.
func NewCLIAuthHandler(runtime sandbox.Runtime, allowedOrigins []string) *CLIAuthHandler {
	return &CLIAuthHandler{term: newTerminal(runtime, "auto-code-os-auth", allowedOrigins, func(h *wsgorilla.Handler) {
		// The provider comes from the ticket, not from a query parameter (REQ-007).
		h.Banner = func(t ticket.Ticket) string {
			return fmt.Sprintf("\r\n🚀 Starting %s Sandbox for authentication...\r\nType your login command (e.g., 'claude login').\r\nFiles saved to /workspace will be automatically captured.\r\n", t.Value)
		}
	})}
}

// MintWSTicket handles POST /organizations/{orgID}/cli-auth/ws-ticket.
// Runs behind the existing AuthMiddleware chi.Router group (Bearer-only).
func (h *CLIAuthHandler) MintWSTicket(w http.ResponseWriter, r *http.Request) {
	h.term.mintTicket(w, r, func(r *http.Request) (string, bool) {
		var body struct {
			Provider string `json:"provider"`
		}
		if err := decodeJSON(r, &body); err != nil || body.Provider == "" {
			return "", false
		}
		return body.Provider, true
	}, "missing provider", func(w http.ResponseWriter, token string) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ticket":     token,
			"expires_in": int(ticket.DefaultTTL.Seconds()),
		})
	})
}

// Terminal handles GET /organizations/{orgID}/cli-auth/terminal via WebSocket.
func (h *CLIAuthHandler) Terminal(w http.ResponseWriter, r *http.Request) {
	h.term.serve(w, r)
}
