package observability

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TraceMiddleware wraps the handler with OpenTelemetry HTTP instrumentation and exposes the trace ID
// in the Trace-ID response header. The X-Request-ID header is set earlier by httpmw.RequestID.
func TraceMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceID(r.Context())
			if traceID != "" {
				w.Header().Set("Trace-ID", traceID)
			}
			next.ServeHTTP(w, r)
		}), serviceName)
	}
}
