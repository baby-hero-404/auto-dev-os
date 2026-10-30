package chat

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/baby-hero-404/aicli/conversation"
	"github.com/baby-hero-404/aicli/profile"
	"github.com/baby-hero-404/aicli/run"
	"github.com/baby-hero-404/aicli/sandbox"
)

// Credentials is the organization's provider-credential store (service.CredentialPoolService).
type Credentials interface {
	ListByOrg(ctx context.Context, orgID string) ([]models.ProviderCredentialResponse, error)
	GetDecryptedCredential(ctx context.Context, orgID, id string) (string, map[string]string, error)
	UpdateCredentialPayload(ctx context.Context, orgID, id string, payload map[string]string) error
}

// Principal is the identity conversations are owned by: a user inside an organization, so the same
// user ID in two organizations never shares conversations, and the Resolver knows whose
// credentials to use.
func Principal(orgID, userID string) conversation.Principal {
	return conversation.Principal(orgID + ":" + userID)
}

func splitPrincipal(p conversation.Principal) (orgID, userID string, ok bool) {
	orgID, userID, ok = strings.Cut(string(p), ":")
	return orgID, userID, ok && orgID != "" && userID != ""
}

// Resolver decides how a chat turn runs. Everything comes from the server: the workspace and the
// CLI session live under DataDir, one pair per conversation, and the login is the organization's
// stored credential for the profile's provider. Nothing comes from the client.
type Resolver struct {
	creds   Credentials
	dataDir string

	mu     sync.Mutex
	leases map[string]lease // conversation ID -> credential the running turn started with
}

type lease struct{ orgID, credentialID string }

// NewResolver returns a Resolver. dataDir must be an absolute path the sandbox runtime can bind.
func NewResolver(creds Credentials, dataDir string) *Resolver {
	return &Resolver{creds: creds, dataDir: dataDir, leases: map[string]lease{}}
}

var _ conversation.Resolver = (*Resolver)(nil)

// Resolve implements conversation.Resolver.
func (r *Resolver) Resolve(ctx context.Context, p conversation.Principal, c conversation.Conversation) (run.Request, error) {
	orgID, _, ok := splitPrincipal(p)
	if !ok {
		return run.Request{}, errors.New("chat: principal has no organization")
	}
	prof, ok := profile.Lookup(c.Profile)
	if !ok {
		return run.Request{}, fmt.Errorf("chat: unknown profile %q", c.Profile)
	}
	workspace := filepath.Join(r.dataDir, "workspaces", c.ID)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return run.Request{}, fmt.Errorf("chat: create workspace: %w", err)
	}
	// One session directory per conversation: the CLI keeps the session later turns resume.
	session, err := sandbox.SessionDir(filepath.Join(r.dataDir, "sessions"), c.Profile, c.ID)
	if err != nil {
		return run.Request{}, err
	}
	credID, files, err := r.pick(ctx, orgID, prof.CredentialProvider)
	if err != nil {
		return run.Request{}, err
	}
	r.mu.Lock()
	if credID != "" {
		r.leases[c.ID] = lease{orgID: orgID, credentialID: credID}
	} else {
		delete(r.leases, c.ID)
	}
	r.mu.Unlock()
	// With no usable credential the CLI starts logged out and the turn ends with auth_invalid,
	// which the UI explains.
	return run.Request{
		Workspace: workspace, SessionDir: session, CredentialFiles: files, TaskID: c.ID,
		Timeout: 30 * time.Minute, IdleTimeout: 5 * time.Minute,
	}, nil
}

// pick returns the organization's first active credential of the provider (the list is ordered by
// priority) with its decrypted files, keyed relative to HOME.
func (r *Resolver) pick(ctx context.Context, orgID, provider string) (string, map[string]string, error) {
	if provider == "" {
		return "", nil, nil
	}
	creds, err := r.creds.ListByOrg(ctx, orgID)
	if err != nil {
		return "", nil, fmt.Errorf("chat: list credentials: %w", err)
	}
	now := time.Now()
	for _, c := range creds {
		if c.Provider != provider || c.Status != models.ProviderCredentialStatusActive || !c.Configured {
			continue
		}
		if c.CooldownUntil != nil && c.CooldownUntil.After(now) {
			continue
		}
		_, files, err := r.creds.GetDecryptedCredential(ctx, orgID, c.ID)
		if err != nil {
			return "", nil, fmt.Errorf("chat: load credential: %w", err)
		}
		return c.ID, files, nil
	}
	return "", nil, nil
}

// SaveCredentials implements conversation.Resolver: files the CLI refreshed during the turn are
// merged onto the credential's current stored state (another run may have refreshed a different
// file meanwhile), so the next turn does not start from an expired token.
func (r *Resolver) SaveCredentials(ctx context.Context, _ conversation.Principal, c conversation.Conversation, files map[string]string) error {
	r.mu.Lock()
	l, ok := r.leases[c.ID]
	delete(r.leases, c.ID)
	r.mu.Unlock()
	if !ok {
		return errors.New("chat: no stored credential for this turn")
	}
	_, current, err := r.creds.GetDecryptedCredential(ctx, l.orgID, l.credentialID)
	if err != nil {
		return fmt.Errorf("chat: reload credential: %w", err)
	}
	merged := make(map[string]string, len(current))
	maps.Copy(merged, current)
	maps.Copy(merged, files)
	return r.creds.UpdateCredentialPayload(ctx, l.orgID, l.credentialID, merged)
}
