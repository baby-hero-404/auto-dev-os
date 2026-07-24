package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// wsSendJSON marshals v and writes it to conn as a text message, discarding
// marshal errors (the payloads here are always static maps) but propagating
// write/connection errors to the caller.
func wsSendJSON(conn *websocket.Conn, v interface{}) error {
	payload, _ := json.Marshal(v)
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func sendWSStdout(conn *websocket.Conn, format string, args ...interface{}) {
	_ = wsSendJSON(conn, map[string]interface{}{
		"type": "stdout",
		"data": fmt.Sprintf(format, args...),
	})
}

func sendWSError(conn *websocket.Conn, format string, args ...interface{}) {
	_ = wsSendJSON(conn, map[string]interface{}{
		"type":    "error",
		"message": fmt.Sprintf(format, args...),
	})
}

// newTerminalWorkspace creates a fresh per-session task ID and temp
// workspace directory under os.TempDir()/<prefix>/<taskID>. The returned
// cleanup func removes the directory and must be deferred by the caller.
func newTerminalWorkspace(prefix string) (taskID, tmpDir string, cleanup func(), err error) {
	taskID = uuid.New().String()
	tmpDir = filepath.Join(os.TempDir(), prefix, taskID)
	if err = os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", "", nil, err
	}
	return taskID, tmpDir, func() { _ = os.RemoveAll(tmpDir) }, nil
}

// runPTYTerminal wires a websocket connection up to an interactive sandbox
// PTY session: stdin/resize frames flow from the socket to the runtime,
// stdout flows back as "stdout" JSON messages. It runs until the sandbox
// command exits or the connection errors.
func runPTYTerminal(ctx context.Context, runtime sandbox.Runtime, conn *websocket.Conn, taskID, tmpDir string) error {
	resizeCh := make(chan sandbox.TerminalSize, 4)
	req := sandbox.CommandRequest{
		TaskID:      taskID,
		Workspace:   tmpDir,
		Command:     []string{"/bin/bash"},
		NetworkMode: sandbox.NetworkModeBridge,
		Env: map[string]string{
			"TERM": "xterm",
			"HOME": "/workspace",
		},
		ResizeCh: resizeCh,
	}

	reader := &wsReader{
		conn: conn,
		onResize: func(cols, rows uint) {
			select {
			case resizeCh <- sandbox.TerminalSize{Cols: cols, Rows: rows}:
			default:
			}
		},
	}
	writer := &wsWriter{conn: conn}

	return runtime.RunInteractive(ctx, req, reader, writer, writer)
}
