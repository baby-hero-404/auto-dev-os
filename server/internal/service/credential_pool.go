package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"sync/atomic"
)

var ErrNoCredentialsAvailable = errors.New("no provider credentials available")

type CredentialStrategy string

const (
	StrategyFillFirst  CredentialStrategy = "fill_first"
	StrategyRoundRobin CredentialStrategy = "round_robin"
)

const (
	testOpenAIModel    = "gpt-5.4-mini"
	testAnthropicModel = "claude-haiku-4-5"
	testGeminiModel    = "gemini-2.5-flash"
	testNineRouterMode = "balanced"
)

type DecryptedCredential struct {
	ID       string
	Provider string
	APIKey   string
	BaseURL  string
}

type providerModelSeeder interface {
	ListByOrg(ctx context.Context, orgID string, filter models.ProviderModelFilter) ([]models.ProviderModel, error)
	Create(ctx context.Context, orgID string, input models.CreateProviderModelInput) (*models.ProviderModel, error)
}

// cooldownCacheTTL bounds how long a replica's in-process view of a (credential, model)
// cooldown can be stale relative to the persisted store before it's refreshed. This lets
// SelectCredential avoid a DB round-trip on every call while still letting a cooldown set
// by one replica become visible to others within a bounded, documented window (REQ-M04).
const cooldownCacheTTL = 15 * time.Second

type cooldownCacheEntry struct {
	until     time.Time
	fetchedAt time.Time
}

type CredentialPoolService struct {
	repo             *repository.ProviderCredentialRepo
	cipher           *SecretCipher
	audit            *AuditService
	connectionTester credentialConnectionTester
	seeder           providerModelSeeder
	mu               sync.Mutex
	rrCounters       map[string]int
	modelCooldowns   map[string]cooldownCacheEntry
	recoveryCounter  int64
}

type credentialConnectionTester func(context.Context, models.ProviderCredential, string) error

func NewCredentialPoolService(repo *repository.ProviderCredentialRepo, cipher *SecretCipher) *CredentialPoolService {
	return &CredentialPoolService{
		repo:             repo,
		cipher:           cipher,
		connectionTester: testProviderConnection,
		rrCounters:       map[string]int{},
		modelCooldowns:   map[string]cooldownCacheEntry{},
	}
}

func (s *CredentialPoolService) WithProviderModelSeeder(seeder providerModelSeeder) *CredentialPoolService {
	s.seeder = seeder
	return s
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
	if s.seeder != nil {
		s.seedDefaultModels(ctx, orgID, cred.Provider)
	}
	resp := cred.ToResponse(suffix)
	return &resp, nil
}

func (s *CredentialPoolService) seedDefaultModels(ctx context.Context, orgID string, provider string) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	filter := models.ProviderModelFilter{
		Provider: &provider,
	}
	existing, err := s.seeder.ListByOrg(ctx, orgID, filter)
	if err != nil {
		slog.Error("failed to list existing provider models during seeding", "org_id", orgID, "provider", provider, "error", err)
		return
	}
	if len(existing) > 0 {
		return
	}

	var defaults []models.CreateProviderModelInput
	switch provider {
	case "openai":
		defaults = []models.CreateProviderModelInput{
			{Provider: "openai", LevelGroup: "fast", ModelName: "gpt-5.4-nano", Priority: 0},
			{Provider: "openai", LevelGroup: "fast", ModelName: "gpt-5.4-mini", Priority: 1},
			{Provider: "openai", LevelGroup: "balanced", ModelName: "gpt-5-mini", Priority: 0},
			{Provider: "openai", LevelGroup: "balanced", ModelName: "gpt-5.4", Priority: 1},
			{Provider: "openai", LevelGroup: "powerful", ModelName: "gpt-5.4-pro", Priority: 0},
			{Provider: "openai", LevelGroup: "powerful", ModelName: "gpt-5.5", Priority: 1},
			{Provider: "openai", LevelGroup: "powerful", ModelName: "gpt-5.5-pro", Priority: 2},
		}
	case "anthropic":
		defaults = []models.CreateProviderModelInput{
			{Provider: "anthropic", LevelGroup: "fast", ModelName: "claude-haiku-4.5", Priority: 0},
			{Provider: "anthropic", LevelGroup: "balanced", ModelName: "claude-sonnet-4.6", Priority: 0},
			{Provider: "anthropic", LevelGroup: "powerful", ModelName: "claude-opus-4.6", Priority: 0},
			{Provider: "anthropic", LevelGroup: "powerful", ModelName: "claude-opus-4.8", Priority: 1},
		}
	case "gemini":
		defaults = []models.CreateProviderModelInput{
			{Provider: "gemini", LevelGroup: "fast", ModelName: "gemini-2.5-flash-lite", Priority: 0},
			{Provider: "gemini", LevelGroup: "balanced", ModelName: "gemini-2.5-flash", Priority: 0},
			{Provider: "gemini", LevelGroup: "powerful", ModelName: "gemini-2.5-pro", Priority: 0},
		}
	}

	for _, d := range defaults {
		if _, err := s.seeder.Create(ctx, orgID, d); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				slog.Debug("default model seed skipped: unique constraint violation", "org_id", orgID, "provider", provider, "model", d.ModelName)
			} else {
				slog.Error("failed to seed default model", "org_id", orgID, "provider", provider, "model", d.ModelName, "error", err)
			}
		}
	}
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

func (s *CredentialPoolService) Update(ctx context.Context, orgID string, id string, input models.UpdateProviderCredentialInput) (*models.ProviderCredentialResponse, error) {
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
	cred, err := s.repo.Update(ctx, orgID, id, input)
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

func (s *CredentialPoolService) Delete(ctx context.Context, orgID string, id string) error {
	cred, err := s.repo.GetByIDAndOrg(ctx, orgID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, orgID, id); err != nil {
		return err
	}
	s.recordAudit(ctx, models.AuditActionProviderCredentialDeleted, cred, map[string]any{
		"provider": cred.Provider,
		"label":    cred.Label,
	})
	return nil
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

func (s *CredentialPoolService) GetRecoveryCount() int64 {
	return atomic.LoadInt64(&s.recoveryCounter)
}
