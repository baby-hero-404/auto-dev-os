package llm

import "context"

const routeTraceKey contextKey = "llm_route_trace"

// RouteTrace records routing decisions that happen deep inside the gateway
// (e.g. AIGateway.ChatWithOptions) that the caller needs to observe afterwards,
// mirroring the prompts.BudgetTrace pattern: a mutable struct stored in ctx,
// written by the callee, read back by the caller once the call completes.
type RouteTrace struct {
	// SelfReviewFallback is true when ExcludeModelID was requested but no
	// alternative model was available, so the excluded model was reused anyway.
	SelfReviewFallback bool
	// ExcludedModel is the model ID that was requested to be excluded.
	ExcludedModel string
	// ActualModel is the model actually used for the call.
	ActualModel string
}

// WithRouteTrace attaches a new RouteTrace to ctx and returns both the
// annotated context and a pointer to the trace for the caller to inspect
// after the call completes.
func WithRouteTrace(ctx context.Context) (context.Context, *RouteTrace) {
	trace := &RouteTrace{}
	return context.WithValue(ctx, routeTraceKey, trace), trace
}

// RouteTraceFromCtx returns the RouteTrace attached to ctx, or nil if none was set.
func RouteTraceFromCtx(ctx context.Context) *RouteTrace {
	if t, ok := ctx.Value(routeTraceKey).(*RouteTrace); ok {
		return t
	}
	return nil
}
