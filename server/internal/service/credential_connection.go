package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

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

func testProviderConnection(ctx context.Context, cred models.ProviderCredential, apiKey string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
