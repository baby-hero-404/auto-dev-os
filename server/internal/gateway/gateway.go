package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AIGateway struct {
	fallback          llm.Provider
	credentialPool    credentialPool
	virtualKeyService virtualKeyService
	routeService      routeService
	cfg               *config.Config
	strategy          service.CredentialStrategy
	cooldown          time.Duration
	providerFactory   providerFactory
}

type Options struct {
	FallbackProvider   llm.Provider
	CredentialPool     credentialPool
	VirtualKeyService  virtualKeyService
	ModelRouteService  routeService
	Config             *config.Config
	CredentialStrategy service.CredentialStrategy
	Cooldown           time.Duration
	ProviderFactory    providerFactory
}

type credentialPool interface {
	SelectCredential(context.Context, string, string, service.CredentialStrategy, map[string]bool) (*service.DecryptedCredential, error)
	SetCooldown(context.Context, string, time.Time) error
}

type virtualKeyService interface {
	Validate(context.Context, string, float64) (*models.VirtualKey, error)
	RecordUsage(context.Context, string, float64) error
}

type routeService interface {
	ResolveRoute(context.Context, string, string, string) (*service.ResolvedRoute, error)
}

type providerFactory func(*service.DecryptedCredential, string) (llm.Provider, error)

type HTTPStatusError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e HTTPStatusError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("http status %d", e.StatusCode)
}

func (e HTTPStatusError) Unwrap() error {
	return e.Err
}

func (e HTTPStatusError) HTTPStatusCode() int {
	return e.StatusCode
}

func NewAIGateway(opts Options) *AIGateway {
	strategy := opts.CredentialStrategy
	if strategy == "" {
		strategy = service.StrategyFillFirst
	}
	cooldown := opts.Cooldown
	if cooldown == 0 {
		cooldown = 5 * time.Minute
	}
	providerFactory := opts.ProviderFactory
	if providerFactory == nil {
		providerFactory = providerFromCredential
	}
	return &AIGateway{
		fallback:          opts.FallbackProvider,
		credentialPool:    opts.CredentialPool,
		virtualKeyService: opts.VirtualKeyService,
		routeService:      opts.ModelRouteService,
		cfg:               opts.Config,
		strategy:          strategy,
		cooldown:          cooldown,
		providerFactory:   providerFactory,
	}
}

func (g *AIGateway) Name() string { return "ai_gateway" }

func (g *AIGateway) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	opts, _ := llm.RouteOptionsFromContext(ctx)
	if opts.OrgID == "" || g.credentialPool == nil {
		return g.chatFallback(ctx, messages)
	}

	entries := g.routeEntries(ctx, opts)
	if len(entries) == 0 {
		return g.chatFallback(ctx, messages)
	}

	var virtualKey *models.VirtualKey
	if opts.VirtualKey != "" && g.virtualKeyService != nil {
		estimated := estimateCost(messages, opts, entries[0])
		key, err := g.virtualKeyService.Validate(ctx, opts.VirtualKey, estimated)
		if err != nil {
			return nil, err
		}
		virtualKey = key
	}

	var failures []string
	for _, entry := range entries {
		excluded := map[string]bool{}
		for {
			cred, err := g.credentialPool.SelectCredential(ctx, opts.OrgID, entry.Provider, g.strategy, excluded)
			if errors.Is(err, service.ErrNoCredentialsAvailable) {
				failures = append(failures, fmt.Sprintf("%s/%s: no credentials", entry.Provider, entry.Model))
				break
			}
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %v", entry.Provider, entry.Model, err))
				break
			}

			provider, err := g.providerFactory(cred, entry.Model)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %v", entry.Provider, entry.Model, err))
				break
			}
			resp, err := provider.Chat(ctx, messages)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s[%s]: %v", entry.Provider, entry.Model, cred.ID, err))
				if isRateLimitError(err) {
					_ = g.credentialPool.SetCooldown(ctx, cred.ID, time.Now().Add(g.cooldown))
					excluded[cred.ID] = true
					continue
				}
				break
			}
			if resp.Model == "" {
				resp.Model = entry.Model
			}
			if virtualKey != nil && g.virtualKeyService != nil {
				meta := llm.MetadataForModel(entry.Provider, resp.Model)
				cost := llm.EstimateCost(resp.PromptTokens, resp.OutputTokens, meta)
				if err := g.virtualKeyService.RecordUsage(ctx, virtualKey.ID, cost); err != nil {
					return nil, err
				}
			}
			return resp, nil
		}
	}

	if opts.VirtualKey == "" {
		if resp, err := g.chatFallback(ctx, messages); err == nil {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("ai gateway exhausted routes: %s", strings.Join(failures, "; "))
}

func (g *AIGateway) chatFallback(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	if g.fallback == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	return g.fallback.Chat(ctx, messages)
}

func (g *AIGateway) routeEntries(ctx context.Context, opts llm.RouteOptions) []models.ComboEntry {
	if g.routeService != nil {
		route, err := g.routeService.ResolveRoute(ctx, opts.OrgID, opts.RouteName, opts.Complexity)
		if err == nil && route != nil && len(route.Entries) > 0 {
			return route.Entries
		}
	}
	return defaultEntries(g.cfg, opts.Complexity)
}

func defaultEntries(cfg *config.Config, complexity string) []models.ComboEntry {
	if cfg == nil {
		return nil
	}
	tier := tierForComplexity(complexity)
	entries := []models.ComboEntry{}
	add := func(provider, model string, priority int) {
		if model != "" {
			entries = append(entries, models.ComboEntry{Provider: provider, Model: model, Priority: priority, Tier: tier})
		}
	}
	switch tier {
	case llm.TierFast:
		add("openai", cfg.LLM.FastModel, 0)
		add("gemini", cfg.LLM.GeminiFastModel, 1)
	case llm.TierPowerful:
		add("openai", cfg.LLM.PowerfulModel, 0)
		add("anthropic", cfg.LLM.AnthropicPowerfulModel, 1)
	default:
		add("openai", cfg.LLM.BalancedModel, 0)
		add("anthropic", cfg.LLM.AnthropicBalancedModel, 1)
		add("gemini", cfg.LLM.GeminiBalancedModel, 2)
	}
	if len(entries) == 0 && cfg.LLM.Provider != "gateway" && cfg.LLM.Model != "" {
		entries = append(entries, models.ComboEntry{Provider: cfg.LLM.Provider, Model: cfg.LLM.Model})
	}
	return entries
}

func providerFromCredential(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
	switch cred.Provider {
	case "openai":
		return llm.NewOpenAI(cred.APIKey, model), nil
	case "anthropic":
		return llm.NewAnthropic(cred.APIKey, model), nil
	case "gemini":
		return llm.NewGemini(cred.APIKey, model), nil
	case "9router":
		return llm.NewNineRouter(cred.APIKey, model, cred.BaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cred.Provider)
	}
}

func estimateCost(messages []llm.Message, opts llm.RouteOptions, entry models.ComboEntry) float64 {
	outputTokens := opts.MaxOutputTokens
	if outputTokens == 0 {
		outputTokens = 2048
	}
	meta := llm.MetadataForModel(entry.Provider, entry.Model)
	return llm.EstimateCost(llm.EstimateMessageTokens(messages), outputTokens, meta)
}

func isRateLimitError(err error) bool {
	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(err, &statusErr) {
		statusCode := statusErr.HTTPStatusCode()
		return statusCode == 429 || statusCode == 402
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 429") || strings.Contains(msg, "status 402") ||
		strings.Contains(msg, "rate limit") || strings.Contains(msg, "quota")
}

func tierForComplexity(complexity string) string {
	switch strings.ToLower(complexity) {
	case models.TaskComplexityEasy:
		return llm.TierFast
	case models.TaskComplexityHard:
		return llm.TierPowerful
	default:
		return llm.TierBalanced
	}
}
