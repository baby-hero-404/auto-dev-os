package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type ctxKey string

const (
	taskIDKey  ctxKey = "task_id"
	agentIDKey ctxKey = "agent_id"
)

func WithTaskID(ctx context.Context, taskID string) context.Context {
	if taskID == "" {
		return ctx
	}
	return context.WithValue(ctx, taskIDKey, taskID)
}

func WithAgentID(ctx context.Context, agentID string) context.Context {
	if agentID == "" {
		return ctx
	}
	return context.WithValue(ctx, agentIDKey, agentID)
}

func TaskID(ctx context.Context) string {
	if taskID, ok := ctx.Value(taskIDKey).(string); ok {
		return taskID
	}
	return ""
}

func TraceID(ctx context.Context) string {
	span := trace.SpanContextFromContext(ctx)
	if !span.HasTraceID() {
		return ""
	}
	return span.TraceID().String()
}

func LogAttrs(ctx context.Context, attrs ...any) []any {
	out := make([]any, 0, len(attrs)+6)
	if traceID := TraceID(ctx); traceID != "" {
		out = append(out, "trace_id", traceID)
	}
	if taskID, ok := ctx.Value(taskIDKey).(string); ok && taskID != "" {
		out = append(out, "task_id", taskID)
	}
	if agentID, ok := ctx.Value(agentIDKey).(string); ok && agentID != "" {
		out = append(out, "agent_id", agentID)
	}
	out = append(out, attrs...)
	return out
}

func Info(ctx context.Context, msg string, attrs ...any) {
	slog.Info(msg, LogAttrs(ctx, attrs...)...)
}

func Warn(ctx context.Context, msg string, attrs ...any) {
	slog.Warn(msg, LogAttrs(ctx, attrs...)...)
}

func Error(ctx context.Context, msg string, attrs ...any) {
	slog.Error(msg, LogAttrs(ctx, attrs...)...)
}
