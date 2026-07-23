package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type CLIAuthHandler struct {
	runtime sandbox.Runtime
	tickets *wsTicketStore
}

func NewCLIAuthHandler(runtime sandbox.Runtime) *CLIAuthHandler {
	return &CLIAuthHandler{runtime: runtime, tickets: newWSTicketStore()}
}

// MintWSTicket handles POST /organizations/{orgID}/cli-auth/ws-ticket.
// Runs behind the existing AuthMiddleware chi.Router group (Bearer-only).
func (h *CLIAuthHandler) MintWSTicket(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	claims, ok := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if !ok || claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "org mismatch")
		return
	}

	var body struct {
		Provider string `json:"provider"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Provider == "" {
		writeError(w, http.StatusBadRequest, "missing provider")
		return
	}

	ticket, err := h.tickets.Mint(claims.Subject, claims.OrgID, body.Provider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint ticket")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket":     ticket,
		"expires_in": int(wsTicketTTL.Seconds()),
	})
}

// wsReader wraps a websocket connection to implement io.Reader for Docker PTY
type wsReader struct {
	conn *websocket.Conn
	buf  []byte
}

func (r *wsReader) Read(p []byte) (n int, err error) {
	if len(r.buf) > 0 {
		n = copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	_, msg, err := r.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	// Assume the incoming message is raw stdin data.
	n = copy(p, msg)
	if n < len(msg) {
		r.buf = msg[n:]
	}
	return n, nil
}

// wsWriter wraps a websocket connection to implement io.Writer for Docker PTY
type wsWriter struct {
	conn *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	// Wrap stdout output in a JSON message so frontend can distinguish it
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "stdout",
		"data": string(p),
	})
	err = w.conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (h *CLIAuthHandler) Terminal(w http.ResponseWriter, r *http.Request) {
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
	provider := ticket.Provider // derived from ticket, not query param (REQ-007)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket: %v", err)
		return
	}
	defer conn.Close()

	sendError := func(msg string) {
		payload, _ := json.Marshal(map[string]interface{}{"type": "error", "message": msg})
		_ = conn.WriteMessage(websocket.TextMessage, payload)
	}

	taskID := uuid.New().String()
	tmpDir := filepath.Join(os.TempDir(), "auto-code-os-auth", taskID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		sendError(fmt.Sprintf("failed to create temp workspace: %v", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	cmd := []string{"/bin/bash"}

	welcomeMsg, _ := json.Marshal(map[string]interface{}{
		"type": "stdout",
		"data": fmt.Sprintf("\r\n🚀 Starting %s Sandbox for authentication...\r\nType your login command (e.g., 'claude login').\r\nFiles saved to /workspace will be automatically captured.\r\n", provider),
	})
	_ = conn.WriteMessage(websocket.TextMessage, welcomeMsg)

	req := sandbox.CommandRequest{
		TaskID:      taskID,
		Workspace:   tmpDir,
		Command:     cmd,
		NetworkMode: sandbox.NetworkModeBridge,
		Env: map[string]string{
			"TERM": "xterm",
		},
	}

	reader := &wsReader{conn: conn}
	writer := &wsWriter{conn: conn}

	err = h.runtime.RunInteractive(context.Background(), req, reader, writer, writer)
	if err != nil {
		sendError(fmt.Sprintf("sandbox error: %v", err))
		return
	}

	resultData := make(map[string]string)
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		content, _ := os.ReadFile(path)
		relPath, _ := filepath.Rel(tmpDir, path)
		resultData[relPath] = string(content)
		return nil
	})

	finalMsg, _ := json.Marshal(map[string]interface{}{
		"type": "stdout",
		"data": "\r\n✅ Session ended. Packaging credential...\r\n",
	})
	_ = conn.WriteMessage(websocket.TextMessage, finalMsg)

	exitMsg, _ := json.Marshal(map[string]interface{}{
		"type":    "exit",
		"payload": resultData,
	})
	_ = conn.WriteMessage(websocket.TextMessage, exitMsg)
}
