package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/baby-hero-404/aicli/conversation"
	"github.com/baby-hero-404/aicli/run"
	"github.com/baby-hero-404/aicli/sandbox"
	"github.com/baby-hero-404/aicli/transport/httpapi"
	"gorm.io/gorm"
)

// Config configures the chat.
type Config struct {
	// Profile is the CLI profile conversations use. It must support streaming (antigravity_notools:
	// the CLI gets no tools).
	Profile string
	// DataDir holds the per-conversation workspaces and CLI sessions. Absolute.
	DataDir string
}

// Chat is the running conversation service.
type Chat struct {
	svc *conversation.Service
}

// New builds the chat on rt (the API's sandbox runtime).
func New(cfg Config, db *gorm.DB, creds Credentials, rt sandbox.Runtime) (*Chat, error) {
	if !filepath.IsAbs(cfg.DataDir) {
		return nil, fmt.Errorf("chat: data dir %q must be an absolute path", cfg.DataDir)
	}
	svc, err := conversation.NewService(conversation.Config{
		Runner:     &run.Runner{Runtime: rt},
		Store:      NewStore(db),
		Authorizer: conversation.OwnerOnly{}, // each conversation belongs to its creator
		Resolver:   NewResolver(creds, cfg.DataDir),
		Profiles:   []string{cfg.Profile},
		Logger:     slog.Default(),
	})
	if err != nil {
		return nil, err
	}
	return &Chat{svc: svc}, nil
}

// Handler returns the conversation API. The caller authenticates the request first and puts the
// caller's Principal in the request context with WithPrincipal. Only the browser origins in
// allowedOrigins may call it; a request without an Origin (a script, curl) is allowed because this
// API is authenticated by a bearer token, which a cross-site page cannot attach.
func (c *Chat) Handler(allowedOrigins []string) (http.Handler, error) {
	return httpapi.NewHandler(httpapi.Config{
		Service: c.svc,
		Authenticate: func(r *http.Request) (conversation.Principal, error) {
			if p, ok := PrincipalFrom(r.Context()); ok {
				return p, nil
			}
			return "", errors.New("chat: no authenticated user")
		},
		AllowedOrigins: allowedOrigins,
		AllowNoOrigin:  true,
		Logger:         slog.Default(),
	})
}

// Run recovers turns whose instance died, now and every minute, until ctx ends.
func (c *Chat) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		if n, err := c.svc.RecoverStale(ctx); err != nil {
			slog.Warn("chat: recover stale turns", "error", err)
		} else if n > 0 {
			slog.Info("chat: recovered stale turns", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Close interrupts running turns and records them. Call it on shutdown.
func (c *Chat) Close(ctx context.Context) error { return c.svc.Close(ctx) }

type principalKey struct{}

// WithPrincipal returns ctx carrying the authenticated caller.
func WithPrincipal(ctx context.Context, p conversation.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the caller put there by WithPrincipal.
func PrincipalFrom(ctx context.Context) (conversation.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(conversation.Principal)
	return p, ok && p != ""
}
