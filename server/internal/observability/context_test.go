package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceID(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0011223344556677")
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	}))

	if got := TraceID(ctx); got != traceID.String() {
		t.Fatalf("TraceID() = %q, want %q", got, traceID.String())
	}
}

func TestLogAttrs(t *testing.T) {
	ctx := WithAgentID(WithTaskID(context.Background(), "task-1"), "agent-1")
	attrs := LogAttrs(ctx, "extra", "value")

	want := map[any]any{
		"task_id":  "task-1",
		"agent_id": "agent-1",
		"extra":    "value",
	}
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			delete(want, attrs[i])
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing attrs: %v in %v", want, attrs)
	}
}
