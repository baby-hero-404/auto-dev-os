package handler

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/baby-hero-404/aicli/capture/ticket"
	"github.com/baby-hero-404/aicli/capture/wsgorilla"
	"github.com/baby-hero-404/aicli/sandbox"
)

// CLITestHandler opens a sandbox terminal with a stored provider credential in place, so the user
// can try the CLI with it. The credential never travels back to the browser.
type CLITestHandler struct {
	term *terminal
}

// NewCLITestHandler returns the handler. Only the browser origins in allowedOrigins may open the
// terminal socket.
func NewCLITestHandler(runtime sandbox.Runtime, svc ProviderCredentialService, allowedOrigins []string) *CLITestHandler {
	return &CLITestHandler{term: newTerminal(runtime, "auto-code-os-cli-test", allowedOrigins, func(h *wsgorilla.Handler) {
		h.Banner = func(ticket.Ticket) string {
			return "\r\n✅ Workspace prepared with your credential.\r\nType the CLI command (e.g. 'claude') to test it.\r\n"
		}
		// The ticket carries the credential ID (Value) and the organization (Scope).
		h.Seed = func(ctx context.Context, t ticket.Ticket) (map[string]string, error) {
			_, files, err := svc.GetDecryptedCredential(ctx, t.Scope, t.Value)
			if err != nil {
				log.Printf("cli-test: load credential %s: %v", t.Value, err)
				return nil, errors.New("failed to get credential")
			}
			return files, nil
		}
		h.DiscardFiles = true
	})}
}

// MintWSTicket handles POST /organizations/{orgID}/cli-test/ws-ticket.
func (h *CLITestHandler) MintWSTicket(w http.ResponseWriter, r *http.Request) {
	h.term.mintTicket(w, r, func(r *http.Request) (string, bool) {
		var body struct {
			CredentialID string `json:"credential_id"`
		}
		if err := decodeJSON(r, &body); err != nil || body.CredentialID == "" {
			return "", false
		}
		return body.CredentialID, true
	}, "missing credential_id", func(w http.ResponseWriter, token string) {
		writeJSON(w, http.StatusOK, map[string]string{"ticket": token})
	})
}

// Terminal handles GET /organizations/{orgID}/cli-test/terminal via WebSocket.
func (h *CLITestHandler) Terminal(w http.ResponseWriter, r *http.Request) {
	h.term.serve(w, r)
}
