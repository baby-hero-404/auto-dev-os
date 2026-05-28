package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const authClaimsKey contextKey = "auth_claims"

type tokenClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	OrgID   string `json:"org_id"`
	Role    string `json:"role"`
	Type    string `json:"typ"`
	Expires int64  `json:"exp"`
}

// ClaimsFromContext extracts JWT claims injected by the auth middleware.
func ClaimsFromContext(ctx context.Context) *tokenClaims {
	claims, _ := ctx.Value(authClaimsKey).(*tokenClaims)
	return claims
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
					var claims tokenClaims
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
