package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// resolve runs the resolver in front of a handler that reports ClientIP(r).
func resolve(t *testing.T, trusted, remoteAddr string, headers map[string]string) string {
	t.Helper()
	var got string
	h := ClientIPResolver(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIP(r) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestClientIP_ForwardingHeadersOnlyFromTrustedProxies(t *testing.T) {
	const private = "10.0.0.0/8,127.0.0.0/8"
	tests := []struct {
		name    string
		trusted string
		remote  string
		headers map[string]string
		want    string
	}{
		{"no proxies trusted: the peer is the client", "", "203.0.113.9:4000", map[string]string{"X-Forwarded-For": "1.1.1.1"}, "203.0.113.9"},
		{"explicit none", "none", "10.0.0.2:4000", map[string]string{"X-Forwarded-For": "1.1.1.1"}, "10.0.0.2"},
		{"untrusted peer cannot spoof X-Forwarded-For", private, "203.0.113.9:4000", map[string]string{"X-Forwarded-For": "1.1.1.1"}, "203.0.113.9"},
		{"untrusted peer cannot spoof X-Real-IP or True-Client-IP", private, "203.0.113.9:4000", map[string]string{"X-Real-IP": "1.1.1.1", "True-Client-IP": "2.2.2.2"}, "203.0.113.9"},
		{"trusted proxy: the forwarded client is used", private, "10.0.0.2:4000", map[string]string{"X-Forwarded-For": "198.51.100.7"}, "198.51.100.7"},
		{"a client cannot prepend a fake address behind a trusted proxy", private, "10.0.0.2:4000", map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.7"}, "198.51.100.7"},
		{"two trusted hops", private, "10.0.0.2:4000", map[string]string{"X-Forwarded-For": "198.51.100.7, 10.0.0.9"}, "198.51.100.7"},
		{"trusted proxy without the header: the peer", private, "10.0.0.2:4000", nil, "10.0.0.2"},
		{"single IP is accepted as a trusted proxy", "192.0.2.10", "192.0.2.10:1", map[string]string{"X-Forwarded-For": "198.51.100.7"}, "198.51.100.7"},
		{"ipv6 peer without trust", "", "[2001:db8::1]:5000", nil, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(t, tt.trusted, tt.remote, tt.headers); got != tt.want {
				t.Errorf("client ip = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIP_WithoutResolverFallsBackToThePeerHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Errorf("without the resolver only the peer counts, got %q", got)
	}
	req.RemoteAddr = "no-port"
	if got := ClientIP(req); got != "no-port" {
		t.Errorf("unparsable remote address is returned as is, got %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	valid, invalid := ParseTrustedProxies(" 10.0.0.0/8 , 192.0.2.5,not-an-ip,,::1/128")
	if len(valid) != 3 || valid[0] != "10.0.0.0/8" || valid[1] != "192.0.2.5/32" || valid[2] != "::1/128" {
		t.Errorf("valid = %v", valid)
	}
	if len(invalid) != 1 || invalid[0] != "not-an-ip" {
		t.Errorf("invalid = %v", invalid)
	}
	if v, i := ParseTrustedProxies("NONE"); len(v) != 0 || len(i) != 0 {
		t.Errorf("none must yield no prefixes: %v %v", v, i)
	}
}

func TestRateLimit_CannotBeBypassedWithForwardingHeaders(t *testing.T) {
	newHandler := func(trusted string) http.Handler {
		rl := NewRateLimiter(2, 2, time.Hour)
		return ClientIPResolver(trusted)(RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })))
	}
	do := func(h http.Handler, remote, xff string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("an internet client rotating X-Forwarded-For and source ports is still limited", func(t *testing.T) {
		h := newHandler("10.0.0.0/8")
		var limited bool
		for i := 0; i < 6; i++ {
			// new source port and a different spoofed address on every request
			code := do(h, "203.0.113.9:"+string(rune('0'+i))+"000", "9.9.9."+string(rune('0'+i)))
			limited = limited || code == http.StatusTooManyRequests
		}
		if !limited {
			t.Fatal("rate limit bypassed by rotating ports and X-Forwarded-For")
		}
	})

	t.Run("behind a trusted proxy each real client has its own bucket", func(t *testing.T) {
		h := newHandler("10.0.0.0/8")
		for _, client := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
			for i := 0; i < 2; i++ {
				if code := do(h, "10.0.0.2:4000", client); code != http.StatusOK {
					t.Fatalf("client %s request %d: %d, want 200", client, i, code)
				}
			}
			if code := do(h, "10.0.0.2:4000", client); code != http.StatusTooManyRequests {
				t.Errorf("client %s third request: %d, want 429", client, code)
			}
		}
	})
}
