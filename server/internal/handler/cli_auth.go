package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type CLIAuthHandler struct {
	runtime sandbox.Runtime
	tickets *wsTerminalTicketStore
}

func NewCLIAuthHandler(runtime sandbox.Runtime) *CLIAuthHandler {
	return &CLIAuthHandler{runtime: runtime, tickets: newWSTerminalTicketStore()}
}

// MintWSTicket handles POST /organizations/{orgID}/cli-auth/ws-ticket.
// Runs behind the existing AuthMiddleware chi.Router group (Bearer-only).
func (h *CLIAuthHandler) MintWSTicket(w http.ResponseWriter, r *http.Request) {
	h.tickets.mintTicket(w, r, func(r *http.Request) (string, bool) {
		var body struct {
			Provider string `json:"provider"`
		}
		if err := decodeJSON(r, &body); err != nil || body.Provider == "" {
			return "", false
		}
		return body.Provider, true
	}, "missing provider", func(w http.ResponseWriter, ticket string) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ticket":     ticket,
			"expires_in": int(wsTicketTTL.Seconds()),
		})
	})
}

// resizeSentinel prefixes an out-of-band terminal-resize control frame sent
// over the same WS connection as raw stdin bytes. It starts with a NUL byte,
// which a real keyboard/xterm.js onData stream never produces, so it can't
// collide with actual stdin content.
const resizeSentinel = "\x00RESIZE:"

// wsReader wraps a websocket connection to implement io.Reader for Docker PTY
// stdin. It also transparently intercepts resizeSentinel control frames and
// forwards them to onResize instead of passing them through as stdin.
type wsReader struct {
	conn     *websocket.Conn
	buf      []byte
	onResize func(cols, rows uint)
}

func (r *wsReader) Read(p []byte) (n int, err error) {
	if len(r.buf) > 0 {
		n = copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	for {
		_, msg, err := r.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if rest, ok := strings.CutPrefix(string(msg), resizeSentinel); ok {
			var cols, rows uint
			if _, scanErr := fmt.Sscanf(rest, "%d:%d", &cols, &rows); scanErr == nil && r.onResize != nil {
				r.onResize(cols, rows)
			}
			continue
		}
		// Assume the incoming message is raw stdin data.
		n = copy(p, msg)
		if n < len(msg) {
			r.buf = msg[n:]
		}
		return n, nil
	}
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
	conn, ticket, taskID, tmpDir, cleanup, ok := h.tickets.beginTerminal(w, r, "auto-code-os-auth")
	if !ok {
		return
	}
	defer conn.Close()
	defer cleanup()

	provider := ticket.Provider // derived from ticket, not query param (REQ-007)

	sendWSStdout(conn, "\r\n🚀 Starting %s Sandbox for authentication...\r\nType your login command (e.g., 'claude login').\r\nFiles saved to /workspace will be automatically captured.\r\n", provider)

	if err := runPTYTerminal(r.Context(), h.runtime, conn, taskID, tmpDir); err != nil {
		sendWSError(conn, "sandbox error: %v", err)
		return
	}

	resultData := make(map[string]string)
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip files larger than 1MB to prevent OOM
		if info.Size() > 1024*1024 {
			return nil
		}
		content, _ := os.ReadFile(path)
		relPath, _ := filepath.Rel(tmpDir, path)
		resultData[relPath] = string(content)
		return nil
	})

	sendWSStdout(conn, "\r\n✅ Session ended. Packaging credential...\r\n")

	_ = wsSendJSON(conn, map[string]interface{}{
		"type":    "exit",
		"payload": resultData,
	})

	// Give a short delay to allow the OS network buffer to flush the exit payload
	// before the TCP connection is aggressively closed by defer conn.Close()
	time.Sleep(500 * time.Millisecond)
}
