package handler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mw "github.com/auto-code-os/auto-code-os/server/internal/middleware"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/baby-hero-404/gocore/stdlog"
	"gorm.io/gorm"
)

func captureStdlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := stdlog.Get()
	stdlog.SetDefault(stdlog.New(stdlog.WithOutput(&buf), stdlog.WithLevel(slog.LevelDebug)))
	t.Cleanup(func() { stdlog.SetDefault(prev) })
	return &buf
}

func TestRouter_RequestIDHeader(t *testing.T) {
	logs := captureStdlog(t)
	router := NewRouter(Deps{})

	tests := []struct {
		name     string
		incoming string
		keep     bool
	}{
		{"valid id is echoed", "client-req-42", true},
		{"missing id is generated", "", false},
		{"unsafe id is replaced", "bad id\r\nSet-Cookie: x=1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			if tt.incoming != "" {
				req.Header.Set("X-Request-ID", tt.incoming)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			got := rr.Header().Get("X-Request-ID")
			if rr.Code != http.StatusOK || got == "" || (got == tt.incoming) != tt.keep {
				t.Fatalf("status=%d X-Request-ID=%q (incoming %q, keep=%v)", rr.Code, got, tt.incoming, tt.keep)
			}
			if rr.Header().Get("Set-Cookie") != "" {
				t.Error("a client supplied request id must never inject headers")
			}
			if !strings.Contains(logs.String(), got) {
				t.Errorf("the request log must carry the request id %q: %s", got, logs.String())
			}
		})
	}
}

func TestWriteServiceError(t *testing.T) {
	logs := captureStdlog(t)
	internal := errors.New("pq: password authentication failed for user \"acos\" at 10.0.0.5:5432")
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
		logged     bool
	}{
		{"invalid input keeps its message", service.ErrInvalid, http.StatusBadRequest, service.ErrInvalid.Error(), false},
		{"not found is fixed text", repository.ErrNotFound, http.StatusNotFound, `"not found"`, false},
		{"gorm not found is fixed text", gorm.ErrRecordNotFound, http.StatusNotFound, `"not found"`, false},
		{"conflict keeps its message", service.ErrConflict, http.StatusConflict, service.ErrConflict.Error(), false},
		{"unexpected error is generic and logged", internal, http.StatusInternalServerError, `"internal server error"`, true},
		{"wrapped unexpected error is generic", errors.Join(errors.New("save task"), internal), http.StatusInternalServerError, `"internal server error"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs.Reset()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/1", nil).WithContext(stdlog.ContextWithRequestID(context.Background(), "rid-77"))
			writeServiceError(rr, req, tt.err)

			if rr.Code != tt.wantStatus || !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Fatalf("got %d %s, want %d containing %s", rr.Code, rr.Body.String(), tt.wantStatus, tt.wantBody)
			}
			if tt.wantStatus >= 500 {
				for _, leak := range []string{"password", "10.0.0.5", "acos"} {
					if strings.Contains(rr.Body.String(), leak) {
						t.Errorf("response leaks %q: %s", leak, rr.Body.String())
					}
				}
			}
			out := logs.String()
			if tt.logged && (!strings.Contains(out, "password authentication failed") || !strings.Contains(out, "rid-77")) {
				t.Errorf("the cause and request id must be logged once: %s", out)
			}
			if !tt.logged && out != "" {
				t.Errorf("client errors must not be logged as failures: %s", out)
			}
		})
	}
}

func TestClientIP_AuditAddressIgnoresSpoofedHeaders(t *testing.T) {
	tests := []struct {
		name    string
		trusted string
		remote  string
		xff     string
		want    string
	}{
		{"internet client cannot choose its audit address", "10.0.0.0/8", "203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},
		{"trusted proxy: the forwarded client is recorded", "10.0.0.0/8", "10.0.0.2:5000", "198.51.100.7", "198.51.100.7"},
		{"no trusted proxy: the peer is recorded", "none", "10.0.0.2:5000", "198.51.100.7", "10.0.0.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := mw.ClientIPResolver(tt.trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = clientIP(r) }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			req.Header.Set("X-Forwarded-For", tt.xff)
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tt.want {
				t.Errorf("audit ip = %q, want %q", got, tt.want)
			}
		})
	}
}
