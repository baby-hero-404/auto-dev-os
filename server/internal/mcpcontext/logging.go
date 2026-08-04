package mcpcontext

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// LogFileName/TraceFileName are the fixed filenames mcp-context writes under
// its log directory (conventionally the container-side end of the
// sandbox.LogsHostDir bind mount, see internal/sandbox/docker.go and
// docs/guides/tracing-workflow-logs.md).
const (
	LogFileName   = "mcp-server.log"
	TraceFileName = "mcp-trace.jsonl"
)

// NewAppLogger builds the *log.Logger mcp-context uses for its own
// Info/Warn/Error application logging. It always writes to stderr — never
// stdout, which is reserved strictly for JSON-RPC responses (a stray log
// line on stdout would corrupt the protocol stream and crash/confuse the
// calling CLI agent). When logDir is non-empty and writable (i.e. the
// sandbox's log bind mount is present), it additionally tees into
// logDir/mcp-server.log via io.MultiWriter, so the log survives container
// removal; if the directory doesn't exist or can't be opened (e.g. running
// locally outside the sandbox, with no bind mount), it silently falls back
// to stderr-only rather than failing the whole process over a
// non-essential diagnostic file.
//
// The returned close func flushes/closes the log file, if one was opened;
// callers should defer it. It is always safe to call, even when no file was
// opened (no-op).
func NewAppLogger(logDir string, stderr io.Writer) (*log.Logger, func()) {
	w, closeFn := openTeeWriter(logDir, LogFileName, stderr)
	return log.New(w, "mcp-context: ", log.LstdFlags), closeFn
}

// OpenTraceWriter opens logDir/mcp-trace.jsonl for append and returns it, or
// (nil, nil) if logDir is empty or the file can't be opened (trace mode is
// strictly best-effort — see cmd/mcp-context's --trace flag). Callers write
// one JSON object per line (Server.SetTrace does this for every JSON-RPC
// request/response pair).
func OpenTraceWriter(logDir string) (io.WriteCloser, error) {
	if logDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(logDir, TraceFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// openTeeWriter returns an io.Writer that always includes fallback, and also
// includes logDir/fileName if that file could be opened for append. The
// second return value closes the file (no-op if none was opened).
func openTeeWriter(logDir, fileName string, fallback io.Writer) (io.Writer, func()) {
	if logDir == "" {
		return fallback, func() {}
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fallback, func() {}
	}
	f, err := os.OpenFile(filepath.Join(logDir, fileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fallback, func() {}
	}
	return io.MultiWriter(fallback, f), func() { _ = f.Close() }
}
