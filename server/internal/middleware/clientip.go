package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ParseTrustedProxies splits a comma separated list of CIDR prefixes (or single IPs) and returns the
// valid ones as prefixes plus the entries that could not be parsed. "none" means no trusted proxy.
func ParseTrustedProxies(csv string) (valid []string, invalid []string) {
	if strings.EqualFold(strings.TrimSpace(csv), "none") {
		return nil, nil
	}
	for _, raw := range strings.Split(csv, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if p, err := netip.ParsePrefix(entry); err == nil {
			valid = append(valid, p.String())
			continue
		}
		if a, err := netip.ParseAddr(entry); err == nil {
			valid = append(valid, netip.PrefixFrom(a, a.BitLen()).String())
			continue
		}
		invalid = append(invalid, entry)
	}
	return valid, invalid
}

// ClientIPResolver returns the middleware that decides who the client is. X-Forwarded-For is
// honoured only when the TCP peer is one of the trusted proxy prefixes, and then only for hops added
// by those proxies; with no trusted proxies the TCP peer is the client. Headers from anyone else are
// ignored, so a client cannot choose the address that is rate limited or written to the audit log.
//
// It replaces chi's RealIP, which copies X-Forwarded-For / X-Real-IP into RemoteAddr from any caller
// (deprecated upstream for exactly that reason).
func ClientIPResolver(trustedProxiesCSV string) func(http.Handler) http.Handler {
	prefixes, _ := ParseTrustedProxies(trustedProxiesCSV)
	if len(prefixes) == 0 {
		return chimw.ClientIPFromRemoteAddr
	}
	return chimw.ClientIPFromXFF(prefixes...)
}

// ClientIP returns the client address resolved by ClientIPResolver, or the TCP peer (without port)
// when that middleware did not run. It never reads forwarding headers itself.
func ClientIP(r *http.Request) string {
	if ip := chimw.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
