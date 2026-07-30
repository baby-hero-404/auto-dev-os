package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
)

type contextKey string

const authClaimsKey contextKey = "auth_claims"

// ClaimsFromContext extracts JWT claims injected by the auth middleware.
func ClaimsFromContext(ctx context.Context) *service.TokenClaims {
	claims, _ := ctx.Value(authClaimsKey).(*service.TokenClaims)
	return claims
}

// WithVerifiedClaims stores signature-verified claims under the same context
// key ClaimsFromContext reads. handler.AuthMiddleware calls this after
// authSvc.VerifyToken succeeds, so RequireRole (and anything else using
// ClaimsFromContext downstream of AuthMiddleware) makes its decision on the
// verified claims rather than the optimistic, unverified ones
// InjectClaimsFromJWT sets early in the chain for rate-limiting purposes.
func WithVerifiedClaims(ctx context.Context, claims *service.TokenClaims) context.Context {
	return context.WithValue(ctx, authClaimsKey, claims)
}

// RequireRole returns middleware that rejects requests from users whose role
// is not in the allowed set. Must be placed after auth middleware.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	set := make(map[string]bool, len(allowed))
	for _, r := range allowed {
		set[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if !set[claims.Role] {
				http.Error(w, `{"error":"forbidden: insufficient role"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// InjectClaimsFromJWT is a best-effort middleware that parses the JWT payload
// (without full verification) and injects claims into the context. This is
// useful for rate-limiting middleware that needs the user ID before the auth
// middleware runs full verification.
func InjectClaimsFromJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			parts := strings.Split(token, ".")
			if len(parts) == 3 {
				if payload, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
					var claims service.TokenClaims
					if json.Unmarshal(payload, &claims) == nil {
						ctx := context.WithValue(r.Context(), authClaimsKey, &claims)
						r = r.WithContext(ctx)
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
