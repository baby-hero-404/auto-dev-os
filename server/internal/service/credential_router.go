package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"log/slog"
)

func (s *CredentialPoolService) SelectCredential(ctx context.Context, orgID, provider, model string, strategy CredentialStrategy, excludeIDs map[string]bool) (*DecryptedCredential, error) {
	creds, err := s.repo.ListActiveByOrgAndProvider(ctx, orgID, strings.ToLower(provider))
	if err != nil {
		return nil, err
	}
	available := make([]models.ProviderCredential, 0, len(creds))
	now := time.Now()
	for _, cred := range creds {
		if excludeIDs != nil && excludeIDs[cred.ID] {
			continue
		}
		if cred.CooldownUntil != nil && cred.CooldownUntil.After(now) {
			continue
		}
		if model != "" {
			cooldownUntil, onCooldown := s.getModelCooldown(ctx, cred.ID, model)
			if onCooldown && cooldownUntil.After(now) {
				continue
			}
		}
		available = append(available, cred)
	}
	if len(available) == 0 {
		return nil, ErrNoCredentialsAvailable
	}
	selected := available[0]
	if strategy == StrategyRoundRobin {
		s.mu.Lock()
		key := orgID + ":" + provider
		idx := s.rrCounters[key] % len(available)
		s.rrCounters[key]++
		s.mu.Unlock()
		selected = available[idx]
	}
	apiKey, err := s.cipher.Decrypt(selected.EncryptedKey)
	if err != nil {
		return nil, err
	}
	apiKey = getAPIKeyOrEnvFallback(selected.Provider, apiKey)
	s.recordAudit(ctx, models.AuditActionProviderCredentialUsed, &selected, map[string]any{
		"provider": selected.Provider,
		"label":    selected.Label,
		"strategy": string(strategy),
	})
	return &DecryptedCredential{
		ID:       selected.ID,
		Provider: selected.Provider,
		APIKey:   apiKey,
		BaseURL:  selected.BaseURL,
	}, nil
}

func (s *CredentialPoolService) SetCooldown(ctx context.Context, id string, model string, until time.Time) error {
	if model != "" {
		// Write-through: persist synchronously so the cooldown survives a restart and is
		// visible to other replicas (REQ-M04). A persistence failure is logged but must not
		// block this replica from still respecting the cooldown locally.
		if err := s.repo.UpsertModelCooldown(ctx, id, model, until); err != nil {
			slog.Error("failed to persist model cooldown, relying on in-process cache only for this replica", "id", id, "model", model, "error", err)
		}

		s.mu.Lock()
		s.modelCooldowns[id+":"+model] = cooldownCacheEntry{until: until, fetchedAt: time.Now()}
		// Prune expired cache entries periodically to prevent map growth
		now := time.Now()
		for k, v := range s.modelCooldowns {
			if now.After(v.until) {
				delete(s.modelCooldowns, k)
			}
		}
		s.mu.Unlock()

		slog.Info("provider credential model-specific rate limited", "id", id, "model", model, "cooldown_until", until)
		return nil
	}

	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.SetCooldown(ctx, id, until); err != nil {
		return err
	}
	s.recordAudit(ctx, models.AuditActionProviderCredentialRateLimited, cred, map[string]any{
		"provider":       cred.Provider,
		"label":          cred.Label,
		"cooldown_until": until,
	})
	return nil
}

func (s *CredentialPoolService) ClearExpiredCooldowns(ctx context.Context) (int64, error) {
	expired, err := s.repo.GetExpiredCooldowns(ctx)
	if err != nil {
		slog.Error("failed to get expired cooldowns for logging", "error", err)
	}

	count, err := s.repo.ClearExpiredCooldowns(ctx)
	if err != nil {
		return 0, err
	}

	if count > 0 && len(expired) > 0 {
		for _, cred := range expired {
			slog.Info(fmt.Sprintf("Credential %s recovered from cooldown, now available", cred.ID))
		}
		atomic.AddInt64(&s.recoveryCounter, int64(len(expired)))
	}

	if count > 0 && s.audit != nil {
		s.audit.RecordAction(ctx, models.AuditActionProviderCredentialRecovered, "provider_credential", "",
			WithDetails(map[string]any{"count": count}),
		)
	}
	return count, nil
}

func (s *CredentialPoolService) GetMinCooldown(ctx context.Context, orgID, provider, model string) (time.Duration, string, error) {
	creds, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return 0, "", err
	}
	now := time.Now()
	var minCooldown time.Duration = -1
	var minCredID string

	for _, cred := range creds {
		if strings.ToLower(cred.Provider) != strings.ToLower(provider) {
			continue
		}
		// Ignore inactive/disabled credentials
		if cred.Status == models.ProviderCredentialStatusDisabled {
			continue
		}

		var cooldownUntil time.Time
		// Check credential-level cooldown
		if cred.Status == models.ProviderCredentialStatusRateLimited && cred.CooldownUntil != nil && cred.CooldownUntil.After(now) {
			cooldownUntil = *cred.CooldownUntil
		}

		// Check model-specific cooldown
		if model != "" {
			mc, ok := s.getModelCooldown(ctx, cred.ID, model)
			if ok && mc.After(now) {
				if mc.After(cooldownUntil) {
					cooldownUntil = mc
				}
			}
		}

		// If this credential has no active cooldown, then its cooldown is 0
		var cd time.Duration
		if !cooldownUntil.IsZero() && cooldownUntil.After(now) {
			cd = cooldownUntil.Sub(now)
		}

		if minCooldown == -1 || cd < minCooldown {
			minCooldown = cd
			minCredID = cred.ID
		}
	}

	if minCooldown == -1 {
		return 0, "", nil
	}
	return minCooldown, minCredID, nil
}

// getModelCooldown returns the current cooldown-until time for a (credential, model) pair
// and whether it's actively on cooldown, reading from the in-process cache when fresh
// (within cooldownCacheTTL) and falling through to the persisted store otherwise (REQ-M04).
// On a persisted-store read error, it falls back to a stale cache entry if one exists, or
// treats the pair as not-on-cooldown (fail open) rather than blocking credential selection.
func (s *CredentialPoolService) getModelCooldown(ctx context.Context, credentialID, model string) (time.Time, bool) {
	key := credentialID + ":" + model

	s.mu.Lock()
	entry, ok := s.modelCooldowns[key]
	fresh := ok && time.Since(entry.fetchedAt) < cooldownCacheTTL
	s.mu.Unlock()

	if fresh {
		return entry.until, entry.until.After(time.Now())
	}

	until, err := s.repo.GetModelCooldown(ctx, credentialID, model)
	if err != nil {
		slog.Warn("failed to read persisted model cooldown, falling back to cache/fail-open", "credential_id", credentialID, "model", model, "error", err)
		if ok {
			return entry.until, entry.until.After(time.Now())
		}
		return time.Time{}, false
	}

	s.mu.Lock()
	s.modelCooldowns[key] = cooldownCacheEntry{until: until, fetchedAt: time.Now()}
	s.mu.Unlock()

	return until, until.After(time.Now())
}
