package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

var ErrNoCredentialsAvailable = errors.New("no provider credentials available")

type CredentialStrategy string

const (
	StrategyFillFirst  CredentialStrategy = "fill_first"
	StrategyRoundRobin CredentialStrategy = "round_robin"
)

type DecryptedCredential struct {
	ID       string
	Provider string
	APIKey   string
	BaseURL  string
}

type CredentialPoolService struct {
	repo             *repository.ProviderCredentialRepo
	cipher           *SecretCipher
	audit            *AuditService
	connectionTester credentialConnectionTester
	mu               sync.Mutex
	rrCounters       map[string]int
}

type credentialConnectionTester func(context.Context, models.ProviderCredential, string) error

func NewCredentialPoolService(repo *repository.ProviderCredentialRepo, cipher *SecretCipher) *CredentialPoolService {
	return &CredentialPoolService{
		repo:             repo,
		cipher:           cipher,
		connectionTester: testProviderConnection,
		rrCounters:       map[string]int{},
	}
}

func (s *CredentialPoolService) WithAudit(audit *AuditService) *CredentialPoolService {
	s.audit = audit
	return s
}

func (s *CredentialPoolService) withConnectionTester(tester credentialConnectionTester) *CredentialPoolService {
	s.connectionTester = tester
	return s
}

func (s *CredentialPoolService) Create(ctx context.Context, orgID string, input models.CreateProviderCredentialInput) (*models.ProviderCredentialResponse, error) {
	if !isAllowedProvider(input.Provider) {
		return nil, ErrValidation("unsupported provider")
	}
	if strings.TrimSpace(input.APIKey) == "" {
		return nil, ErrValidation("api_key is required")
	}
	suffix := keySuffix(input.APIKey)
	encrypted, err := s.cipher.Encrypt(input.APIKey)
	if err != nil {
		return nil, err
	}
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.APIKey = encrypted
	cred, err := s.repo.Create(ctx, orgID, input)
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, models.AuditActionProviderCredentialCreated, cred, map[string]any{
		"provider": cred.Provider,
		"label":    cred.Label,
		"priority": cred.Priority,
	})
	resp := cred.ToResponse(suffix)
	return &resp, nil
}

func (s *CredentialPoolService) GetByID(ctx context.Context, id string) (*models.ProviderCredentialResponse, error) {
	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := cred.ToResponse("")
	return &resp, nil
}

func (s *CredentialPoolService) ListByOrg(ctx context.Context, orgID string) ([]models.ProviderCredentialResponse, error) {
	creds, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]models.ProviderCredentialResponse, 0, len(creds))
	for _, cred := range creds {
		out = append(out, cred.ToResponse(""))
	}
	return out, nil
}

func (s *CredentialPoolService) Update(ctx context.Context, id string, input models.UpdateProviderCredentialInput) (*models.ProviderCredentialResponse, error) {
	if input.APIKey != nil {
		encrypted, err := s.cipher.Encrypt(*input.APIKey)
		if err != nil {
			return nil, err
		}
		input.APIKey = &encrypted
	}
	if input.Status != nil && !isAllowedCredentialStatus(*input.Status) {
		return nil, ErrValidation("unsupported credential status")
	}
	cred, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, models.AuditActionProviderCredentialUpdated, cred, map[string]any{
		"provider": cred.Provider,
		"label":    cred.Label,
		"status":   cred.Status,
		"priority": cred.Priority,
	})
	resp := cred.ToResponse("")
	return &resp, nil
}

func (s *CredentialPoolService) Delete(ctx context.Context, id string) error {
	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.recordAudit(ctx, models.AuditActionProviderCredentialDeleted, cred, map[string]any{
		"provider": cred.Provider,
		"label":    cred.Label,
	})
	return nil
}

func (s *CredentialPoolService) TestConnection(ctx context.Context, id string) error {
	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cred.EncryptedKey == "" {
		return ErrValidation("credential key is empty")
	}
	apiKey, err := s.cipher.Decrypt(cred.EncryptedKey)
	if err != nil {
		return err
	}
	if s.connectionTester != nil {
		if err := s.connectionTester(ctx, *cred, apiKey); err != nil {
			return err
		}
	}
	s.recordAudit(ctx, models.AuditActionProviderCredentialTested, cred, map[string]any{
		"provider": cred.Provider,
		"label":    cred.Label,
	})
	return nil
}

func (s *CredentialPoolService) SelectCredential(ctx context.Context, orgID, provider string, strategy CredentialStrategy, excludeIDs map[string]bool) (*DecryptedCredential, error) {
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

func (s *CredentialPoolService) SetCooldown(ctx context.Context, id string, until time.Time) error {
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
	count, err := s.repo.ClearExpiredCooldowns(ctx)
	if err != nil {
		return 0, err
	}
	if count > 0 && s.audit != nil {
		s.audit.RecordAction(ctx, models.AuditActionProviderCredentialRecovered, "provider_credential", "",
			WithDetails(map[string]any{"count": count}),
		)
	}
	return count, nil
}

func (s *CredentialPoolService) recordAudit(ctx context.Context, action string, cred *models.ProviderCredential, details map[string]any) {
	if s.audit == nil || cred == nil {
		return
	}
	s.audit.RecordAction(ctx, action, "provider_credential", cred.ID,
		WithOrgID(cred.OrgID),
		WithDetails(details),
	)
}

func isAllowedProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "anthropic", "gemini", "9router":
		return true
	default:
		return false
	}
}

func isAllowedCredentialStatus(status string) bool {
	switch status {
	case models.ProviderCredentialStatusActive, models.ProviderCredentialStatusRateLimited, models.ProviderCredentialStatusDisabled:
		return true
	default:
		return false
	}
}

func keySuffix(key string) string {
	if len(key) <= 4 {
		return key
	}
	return key[len(key)-4:]
}

func testProviderConnection(ctx context.Context, cred models.ProviderCredential, apiKey string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	provider, err := providerForCredentialTest(cred, apiKey)
	if err != nil {
		return err
	}
	_, err = provider.Chat(ctx, []llm.Message{{Role: "user", Content: "Reply with OK."}})
	if err != nil {
		return fmt.Errorf("test %s credential: %w", cred.Provider, err)
	}
	return nil
}

func providerForCredentialTest(cred models.ProviderCredential, apiKey string) (llm.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cred.Provider)) {
	case "openai":
		return llm.NewOpenAI(apiKey, "gpt-4o-mini"), nil
	case "anthropic":
		return llm.NewAnthropic(apiKey, "claude-3-5-haiku-latest"), nil
	case "gemini":
		return llm.NewGemini(apiKey, "gemini-1.5-flash"), nil
	case "9router":
		return llm.NewNineRouter(apiKey, "openai/gpt-4o-mini", cred.BaseURL), nil
	default:
		return nil, ErrValidation("unsupported provider")
	}
}
