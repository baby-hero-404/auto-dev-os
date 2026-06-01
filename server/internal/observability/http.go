package observability

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func TraceMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceID(r.Context())
			if traceID != "" {
				w.Header().Set("Trace-ID", traceID)
			}
			if requestID := chimw.GetReqID(r.Context()); requestID != "" {
				w.Header().Set("X-Request-ID", requestID)
			}
			next.ServeHTTP(w, r)
		}), serviceName)
	}
}
