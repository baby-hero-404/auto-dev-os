package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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

const (
	testOpenAIModel    = "gpt-5.4-mini"
	testAnthropicModel = "claude-haiku-4-5"
	testGeminiModel    = "gemini-3.5-flash"
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

type CredentialPoolService struct {
	repo             *repository.ProviderCredentialRepo
	cipher           *SecretCipher
	audit            *AuditService
	connectionTester credentialConnectionTester
	seeder           providerModelSeeder
	mu               sync.Mutex
	rrCounters       map[string]int
	modelCooldowns   map[string]time.Time
}

type credentialConnectionTester func(context.Context, models.ProviderCredential, string) error

func NewCredentialPoolService(repo *repository.ProviderCredentialRepo, cipher *SecretCipher) *CredentialPoolService {
	return &CredentialPoolService{
		repo:             repo,
		cipher:           cipher,
		connectionTester: testProviderConnection,
		rrCounters:       map[string]int{},
		modelCooldowns:   map[string]time.Time{},
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
			{Provider: "gemini", LevelGroup: "fast", ModelName: "gemini-3.1-flash-lite", Priority: 0},
			{Provider: "gemini", LevelGroup: "fast", ModelName: "gemini-2.5-flash-lite", Priority: 1},
			{Provider: "gemini", LevelGroup: "balanced", ModelName: "gemini-3.5-flash", Priority: 0},
			{Provider: "gemini", LevelGroup: "balanced", ModelName: "gemini-2.5-flash", Priority: 1},
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

func (s *CredentialPoolService) TestConnection(ctx context.Context, orgID string, id string) error {
	cred, err := s.repo.GetByIDAndOrg(ctx, orgID, id)
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

func (s *CredentialPoolService) TestConnectionInput(ctx context.Context, input models.TestProviderCredentialInput) error {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if !isAllowedProvider(provider) {
		return ErrValidation("unsupported provider")
	}
	if strings.TrimSpace(input.APIKey) == "" {
		return ErrValidation("api_key is required")
	}
	if s.connectionTester == nil {
		return nil
	}
	return s.connectionTester(ctx, models.ProviderCredential{
		Provider: provider,
		BaseURL:  strings.TrimSpace(input.BaseURL),
	}, strings.TrimSpace(input.APIKey))
}

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
			s.mu.Lock()
			cooldownUntil, onCooldown := s.modelCooldowns[cred.ID+":"+model]
			s.mu.Unlock()
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
		s.mu.Lock()
		s.modelCooldowns[id+":"+model] = until
		// Prune expired cooldowns periodically to prevent map growth
		now := time.Now()
		for k, v := range s.modelCooldowns {
			if now.After(v) {
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

	apiKey = getAPIKeyOrEnvFallback(cred.Provider, apiKey)

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
		return llm.NewOpenAI(apiKey, testOpenAIModel), nil
	case "anthropic":
		return llm.NewAnthropic(apiKey, testAnthropicModel), nil
	case "gemini":
		return llm.NewGemini(apiKey, testGeminiModel), nil
	case "9router":
		return llm.NewNineRouter(apiKey, testNineRouterMode, cred.BaseURL), nil
	default:
		return nil, ErrValidation("unsupported provider")
	}
}

func getAPIKeyOrEnvFallback(provider string, apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	isPlaceholder := apiKey == "" ||
		strings.Contains(apiKey, "your-") ||
		strings.Contains(apiKey, "placeholder") ||
		apiKey == "sk-test"
	if !isPlaceholder {
		return apiKey
	}

	switch strings.ToLower(provider) {
	case "openai":
		if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" && !strings.Contains(envKey, "your-") {
			return envKey
		}
	case "anthropic":
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" && !strings.Contains(envKey, "your-") {
			return envKey
		}
	case "gemini":
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" && !strings.Contains(envKey, "your-") {
			return envKey
		}
	}
	return apiKey
}
