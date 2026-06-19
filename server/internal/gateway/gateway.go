package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AIGateway struct {
	fallback              llm.Provider
	credentialPool        credentialPool
	providerModelResolver ProviderModelResolver
	cfg                   *config.Config
	strategy              service.CredentialStrategy
	cooldown              time.Duration
	providerFactory       providerFactory
	recorder              llm.UsageRecorder
}

type Options struct {
	FallbackProvider      llm.Provider
	CredentialPool        credentialPool
	ProviderModelResolver ProviderModelResolver
	Config                *config.Config
	CredentialStrategy    service.CredentialStrategy
	Cooldown              time.Duration
	ProviderFactory       providerFactory
	Recorder              llm.UsageRecorder
}

type credentialPool interface {
	SelectCredential(context.Context, string, string, service.CredentialStrategy, map[string]bool) (*service.DecryptedCredential, error)
	SetCooldown(context.Context, string, time.Time) error
}

type ProviderModelResolver interface {
	ResolveModels(ctx context.Context, orgID string, levelGroup string) ([]models.ProviderModel, error)
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
		fallback:              opts.FallbackProvider,
		credentialPool:        opts.CredentialPool,
		providerModelResolver: opts.ProviderModelResolver,
		cfg:                   opts.Config,
		strategy:              strategy,
		cooldown:              cooldown,
		providerFactory:       providerFactory,
		recorder:              opts.Recorder,
	}
}

func (g *AIGateway) Name() string { return "ai_gateway" }

func (g *AIGateway) Chat(ctx context.Context, messages []llm.Message) (resp *llm.Response, err error) {
	startTime := time.Now()
	opts, _ := llm.RouteOptionsFromContext(ctx)
	var lastEntry *models.ComboEntry
	var lastCred *service.DecryptedCredential

	defer func() {
		if opts.OrgID == "" || g.recorder == nil {
			return
		}

		record := llm.UsageRecord{
			ProjectID: opts.ProjectID,
			AgentID:   opts.AgentID,
			TaskID:    opts.TaskID,
			OrgID:     opts.OrgID,
			LatencyMS: time.Since(startTime).Milliseconds(),
		}

		var provider, model, levelGroup string
		if lastEntry != nil {
			provider = lastEntry.Provider
			model = lastEntry.Model
			levelGroup = lastEntry.LevelGroup
		} else if g.fallback != nil {
			if metaProv, ok := g.fallback.(llm.MetadataProvider); ok {
				meta := metaProv.Metadata()
				provider = meta.Provider
				model = meta.Model
				levelGroup = meta.LevelGroup
			} else {
				provider = g.fallback.Name()
				if g.cfg != nil {
					model = g.cfg.LLM.Model
				}
			}
		}

		if provider == "" {
			provider = "gateway"
		}
		if model == "" {
			model = "unknown"
		}
		if levelGroup == "" {
			levelGroup = "balanced"
		}

		record.Provider = provider
		record.Model = model
		record.LevelGroup = levelGroup

		if lastCred != nil {
			record.CredentialID = lastCred.ID
		}

		if err != nil {
			record.Status = "failed"
			record.Error = err.Error()
		} else if resp != nil {
			record.Status = "ok"
			if resp.Model != "" {
				record.Model = resp.Model
				model = resp.Model
			}
			record.PromptTokens = resp.PromptTokens
			record.OutputTokens = resp.OutputTokens

			meta := llm.MetadataForModel(provider, model)
			record.CostUSD = llm.EstimateCost(resp.PromptTokens, resp.OutputTokens, meta)
		}

		ctxCopy := context.WithoutCancel(ctx)
		bgCtx, cancel := context.WithTimeout(ctxCopy, 2*time.Second)

		go func() {
			defer cancel()
			if recErr := g.recorder.RecordLLMUsage(bgCtx, record); recErr != nil {
				log.Printf("[AIGateway] Telemetry record failed: %v", recErr)
			}
		}()
	}()

	if opts.OrgID == "" || g.credentialPool == nil {
		resp, err = g.chatFallback(ctx, messages)
		return resp, err
	}

	entries, err := g.routeEntries(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		err = fmt.Errorf("no active models configured for organization %s and level %s", opts.OrgID, getLevelGroup(opts))
		return nil, err
	}

	var failures []string
	for _, entry := range entries {
		currentEntry := entry
		lastEntry = &currentEntry
		excluded := map[string]bool{}
		for {
			var cred *service.DecryptedCredential
			cred, err = g.credentialPool.SelectCredential(ctx, opts.OrgID, entry.Provider, g.strategy, excluded)
			if errors.Is(err, service.ErrNoCredentialsAvailable) {
				failures = append(failures, fmt.Sprintf("%s/%s: no credentials", entry.Provider, entry.Model))
				break
			}
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %v", entry.Provider, entry.Model, err))
				break
			}
			lastCred = cred

			var provider llm.Provider
			provider, err = g.providerFactory(cred, entry.Model)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %v", entry.Provider, entry.Model, err))
				break
			}
			attempts := 4
			for attempt := 0; attempt < attempts; attempt++ {
				resp, err = provider.Chat(ctx, messages)
				if err == nil {
					break
				}
				if isTransientError(err) && attempt < attempts-1 {
					backoff := time.Duration(1000*(1<<attempt)) * time.Millisecond
					log.Printf("[AIGateway] Transient error calling %s/%s: %v. Retrying in %v (attempt %d/%d)...", entry.Provider, entry.Model, err, backoff, attempt+1, attempts)
					timer := time.NewTimer(backoff)
					select {
					case <-ctx.Done():
						timer.Stop()
						err = ctx.Err()
						return nil, err
					case <-timer.C:
					}
					continue
				}
				break
			}
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s[%s]: %v", entry.Provider, entry.Model, cred.ID, err))
				if isTransientError(err) {
					_ = g.credentialPool.SetCooldown(ctx, cred.ID, time.Now().Add(g.cooldown))
					excluded[cred.ID] = true
					continue
				}
				break
			}
			if resp.Model == "" {
				resp.Model = entry.Model
			}
			return resp, nil
		}
	}

	err = fmt.Errorf("ai gateway exhausted routes: %s", strings.Join(failures, "; "))
	return nil, err
}

func (g *AIGateway) chatFallback(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	if g.fallback == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	return g.fallback.Chat(ctx, messages)
}

func (g *AIGateway) routeEntries(ctx context.Context, opts llm.RouteOptions) ([]models.ComboEntry, error) {
	if g.providerModelResolver == nil {
		return nil, fmt.Errorf("provider model resolver is not configured")
	}
	level := getLevelGroup(opts)
	providerModels, err := g.providerModelResolver.ResolveModels(ctx, opts.OrgID, level)
	if err != nil {
		return nil, err
	}
	entries := make([]models.ComboEntry, 0, len(providerModels))
	for _, pm := range providerModels {
		entries = append(entries, models.ComboEntry{
			Provider:   pm.Provider,
			Model:      pm.ModelName,
			Priority:   pm.Priority,
			LevelGroup: pm.LevelGroup,
		})
	}
	return entries, nil
}

func getLevelGroup(opts llm.RouteOptions) string {
	name := strings.ToLower(strings.TrimSpace(opts.RouteName))
	if name == models.ModelLevelFast || name == models.ModelLevelBalanced || name == models.ModelLevelPowerful {
		return name
	}
	return levelGroupForComplexity(opts.Complexity)
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

func isTransientError(err error) bool {
	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(err, &statusErr) {
		statusCode := statusErr.HTTPStatusCode()
		return statusCode == 429 || statusCode == 402 || statusCode == 500 || statusCode == 502 || statusCode == 503 || statusCode == 504
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 429") || strings.Contains(msg, "status 402") ||
		strings.Contains(msg, "status 500") || strings.Contains(msg, "status 502") ||
		strings.Contains(msg, "status 503") || strings.Contains(msg, "status 504") ||
		strings.Contains(msg, "rate limit") || strings.Contains(msg, "quota") ||
		strings.Contains(msg, "temporarily unavailable") || strings.Contains(msg, "high demand")
}

func levelGroupForComplexity(complexity string) string {
	switch strings.ToLower(complexity) {
	case models.TaskComplexityEasy:
		return llm.LevelFast
	case models.TaskComplexityHard:
		return llm.LevelPowerful
	default:
		return llm.LevelBalanced
	}
}
