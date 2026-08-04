package mcpcontext

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppLogger_EmptyLogDir_StderrOnly(t *testing.T) {
	var stderr bytes.Buffer
	logger, closeFn := NewAppLogger("", &stderr)
	defer closeFn()

	logger.Print("hello")
	if !strings.Contains(stderr.String(), "hello") {
		t.Errorf("expected fallback writer to receive the log line, got: %q", stderr.String())
	}
}

func TestNewAppLogger_ValidLogDir_TeesToFile(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	logger, closeFn := NewAppLogger(dir, &stderr)

	logger.Print("hello from test")
	closeFn()

	if !strings.Contains(stderr.String(), "hello from test") {
		t.Errorf("expected stderr to still receive the log line, got: %q", stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, LogFileName))
	if err != nil {
		t.Fatalf("expected %s to exist: %v", LogFileName, err)
	}
	if !strings.Contains(string(data), "hello from test") {
		t.Errorf("expected log file to contain the log line, got: %q", string(data))
	}
}

func TestNewAppLogger_UnwritableLogDir_FallsBackSilently(t *testing.T) {
	// A file path used as a directory can never be mkdir'd into — this
	// exercises the "sandbox bind mount absent" fallback path outside the
	// container.
	dir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(dir, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var stderr bytes.Buffer
	logger, closeFn := NewAppLogger(dir, &stderr)
	defer closeFn()

	logger.Print("still works")
	if !strings.Contains(stderr.String(), "still works") {
		t.Errorf("expected fallback to stderr when log dir is unwritable, got: %q", stderr.String())
	}
}

func TestOpenTraceWriter_EmptyDirReturnsNil(t *testing.T) {
	w, err := OpenTraceWriter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != nil {
		t.Errorf("expected nil writer for empty log dir")
	}
}

func TestOpenTraceWriter_WritesToTraceFile(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenTraceWriter(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatalf("expected a non-nil writer")
	}
	defer w.Close()

	if _, err := w.Write([]byte(`{"hello":"trace"}` + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, TraceFileName))
	if err != nil {
		t.Fatalf("expected %s to exist: %v", TraceFileName, err)
	}
	if !strings.Contains(string(data), "trace") {
		t.Errorf("unexpected trace file content: %q", string(data))
	}
}
