package handler

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
)

type CLITestHandler struct {
	runtime sandbox.Runtime
	svc     ProviderCredentialService
	tickets *wsTicketStore
}

func NewCLITestHandler(runtime sandbox.Runtime, svc ProviderCredentialService) *CLITestHandler {
	return &CLITestHandler{
		runtime: runtime,
		svc:     svc,
		tickets: newWSTicketStore(),
	}
}

// MintWSTicket handles POST /organizations/{orgID}/cli-test/ws-ticket.
func (h *CLITestHandler) MintWSTicket(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	claims, ok := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if !ok || claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "org mismatch")
		return
	}

	var body struct {
		CredentialID string `json:"credential_id"`
	}
	if err := decodeJSON(r, &body); err != nil || body.CredentialID == "" {
		writeError(w, http.StatusBadRequest, "missing credential_id")
		return
	}

	ticket, err := h.tickets.Mint(claims.Subject, claims.OrgID, body.CredentialID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint ticket")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"ticket": ticket})
}

// Terminal handles GET /organizations/{orgID}/cli-test/terminal via WebSocket.
func (h *CLITestHandler) Terminal(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	ticketStr := r.URL.Query().Get("ticket")
	if ticketStr == "" {
		http.Error(w, "missing ticket", http.StatusUnauthorized)
		return
	}
	ticket, ok := h.tickets.Consume(ticketStr, orgID)
	if !ok {
		http.Error(w, "invalid, expired, or already-used ticket", http.StatusUnauthorized)
		return
	}
	credentialID := ticket.Provider // The ticket system stores the requested string in 'Provider'

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket: %v", err)
		return
	}
	defer conn.Close()

	// Fetch decrypted credential
	provider, payloadMap, err := h.svc.GetDecryptedCredential(r.Context(), orgID, credentialID)
	if err != nil {
		sendWSError(conn, "failed to get credential: %v", err)
		return
	}

	taskID, tmpDir, cleanup, err := newTerminalWorkspace("auto-code-os-cli-test")
	if err != nil {
		sendWSError(conn, "failed to create temp workspace: %v", err)
		return
	}
	defer cleanup()

	// Write credential payload to the temp dir based on the JSON payload structure
	for relPath, content := range payloadMap {
		fullPath := filepath.Join(tmpDir, relPath)
		// Ensure no directory traversal
		if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(tmpDir)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(fullPath, []byte(content), 0o600)
	}

	providerName := provider
	// Extract basic tool name (e.g. 'claude' from 'cli:claude')
	if len(providerName) > 4 && providerName[:4] == "cli:" {
		providerName = providerName[4:]
	}

	sendWSStdout(conn, "\r\n✅ Workspace prepared with your %s credential.\r\nType the CLI command (e.g. '%s') to test it.\r\n", provider, providerName)

	if err := runPTYTerminal(r.Context(), h.runtime, conn, taskID, tmpDir); err != nil {
		sendWSError(conn, "sandbox error: %v", err)
		return
	}

	_ = wsSendJSON(conn, map[string]interface{}{"type": "exit"})
}
