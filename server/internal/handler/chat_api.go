package handler

import (
	"log/slog"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/chat"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
)

// ChatService is the conversation API of an organization (internal/chat).
type ChatService interface {
	// Handler returns the conversation API for the given browser origins.
	Handler(allowedOrigins []string) (http.Handler, error)
}

// chatRoute serves /organizations/{orgID}/chat/*: the bearer token was verified by AuthMiddleware;
// this checks the token belongs to the organization in the path, hands the caller's identity to the
// conversation API and strips the prefix. Chat spends the organization's CLI credential, so the
// route is admin-only like the credentials themselves (see the router).
func chatRoute(svc ChatService, allowedOrigins []string) http.HandlerFunc {
	api, err := svc.Handler(allowedOrigins)
	if err != nil {
		slog.Error("chat: conversation API not mounted", "error", err)
		return func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "chat is unavailable")
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "orgID")
		claims, ok := r.Context().Value(authClaimsKey).(*service.TokenClaims)
		if !ok || claims.OrgID != orgID {
			writeError(w, http.StatusForbidden, "org mismatch")
			return
		}
		ctx := chat.WithPrincipal(r.Context(), chat.Principal(orgID, claims.Subject))
		http.StripPrefix("/api/v1/organizations/"+orgID+"/chat", api).ServeHTTP(w, r.WithContext(ctx))
	}
}
