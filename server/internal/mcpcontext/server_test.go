package mcpcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
)

func TestServer_Trace_DisabledByDefault(t *testing.T) {
	var out bytes.Buffer
	srv := NewServer("test", "0.0.1", log.New(&out, "", 0))

	var stdout bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if err := srv.Serve(context.Background(), in, &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no trace output when SetTrace was never called, got: %q", out.String())
	}
}

func TestServer_Trace_WritesRequestResponsePairs(t *testing.T) {
	var trace bytes.Buffer
	srv := NewServer("test", "0.0.1", log.New(&bytes.Buffer{}, "", 0))
	srv.SetTrace(&trace)

	var stdout bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if err := srv.Serve(context.Background(), in, &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(trace.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 trace line, got %d: %q", len(lines), trace.String())
	}

	var ev traceEvent
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("trace line is not valid JSON: %v", err)
	}
	if !strings.Contains(string(ev.Request), `"tools/list"`) {
		t.Errorf("trace request doesn't contain the original method: %s", ev.Request)
	}
	if ev.Response == nil {
		t.Fatalf("expected a non-nil response in the trace event")
	}
}

func TestServer_Trace_NotificationHasNilResponse(t *testing.T) {
	var trace bytes.Buffer
	srv := NewServer("test", "0.0.1", log.New(&bytes.Buffer{}, "", 0))
	srv.SetTrace(&trace)

	var stdout bytes.Buffer
	// No "id" field => notification, no response expected.
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	if err := srv.Serve(context.Background(), in, &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}

	var ev traceEvent
	if err := json.Unmarshal(bytes.TrimSpace(trace.Bytes()), &ev); err != nil {
		t.Fatalf("trace line is not valid JSON: %v", err)
	}
	if ev.Response != nil {
		t.Errorf("expected nil response for a notification, got %+v", ev.Response)
	}
	if stdout.Len() != 0 {
		t.Errorf("notifications must not produce a stdout JSON-RPC reply, got: %q", stdout.String())
	}
}
