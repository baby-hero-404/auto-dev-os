package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

type CLITestHandler struct {
	runtime sandbox.Runtime
	svc     ProviderCredentialService
	tickets *wsTerminalTicketStore
}

func NewCLITestHandler(runtime sandbox.Runtime, svc ProviderCredentialService) *CLITestHandler {
	return &CLITestHandler{
		runtime: runtime,
		svc:     svc,
		tickets: newWSTerminalTicketStore(),
	}
}

// MintWSTicket handles POST /organizations/{orgID}/cli-test/ws-ticket.
func (h *CLITestHandler) MintWSTicket(w http.ResponseWriter, r *http.Request) {
	h.tickets.mintTicket(w, r, func(r *http.Request) (string, bool) {
		var body struct {
			CredentialID string `json:"credential_id"`
		}
		if err := decodeJSON(r, &body); err != nil || body.CredentialID == "" {
			return "", false
		}
		return body.CredentialID, true
	}, "missing credential_id", func(w http.ResponseWriter, ticket string) {
		writeJSON(w, http.StatusOK, map[string]string{"ticket": ticket})
	})
}

// Terminal handles GET /organizations/{orgID}/cli-test/terminal via WebSocket.
func (h *CLITestHandler) Terminal(w http.ResponseWriter, r *http.Request) {
	conn, ticket, taskID, tmpDir, cleanup, ok := h.tickets.beginTerminal(w, r, "auto-code-os-cli-test")
	if !ok {
		return
	}
	defer conn.Close()
	defer cleanup()

	credentialID := ticket.Provider // The ticket system stores the requested string in 'Provider'

	// Fetch decrypted credential
	provider, payloadMap, err := h.svc.GetDecryptedCredential(r.Context(), ticket.OrgID, credentialID)
	if err != nil {
		sendWSError(conn, "failed to get credential: %v", err)
		return
	}

	// Write credential payload to the temp dir based on the JSON payload structure
	for relPath, content := range payloadMap {
		fullPath := filepath.Join(tmpDir, relPath)
		// Ensure no directory traversal
		rel, err := filepath.Rel(tmpDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(fullPath, []byte(content), 0o600)
	}

	providerName := provider
	// Extract basic tool name (e.g. 'claude' from 'cli:claude')
	if rest, ok := strings.CutPrefix(providerName, "cli:"); ok {
		providerName = rest
	}

	sendWSStdout(conn, "\r\n✅ Workspace prepared with your %s credential.\r\nType the CLI command (e.g. '%s') to test it.\r\n", provider, providerName)

	if err := runPTYTerminal(r.Context(), h.runtime, conn, taskID, tmpDir); err != nil {
		sendWSError(conn, "sandbox error: %v", err)
		return
	}

	_ = wsSendJSON(conn, map[string]interface{}{"type": "exit"})
}

// writeCredentialFile writes content to relPath under baseDir, refusing to
// write anywhere relPath would resolve outside baseDir. Matches the
// filepath.Rel-based containment check engine/cli.go uses for the same
// class of untrusted-relative-path problem (a plain filepath.Clean+HasPrefix
// string check is not sufficient: a sibling directory that merely shares
// baseDir's string prefix, e.g. baseDir+"-evil", would incorrectly pass it).
func writeCredentialFile(baseDir, relPath, content string) {
	fullPath := filepath.Join(baseDir, relPath)
	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(fullPath, []byte(content), 0o600)
}
