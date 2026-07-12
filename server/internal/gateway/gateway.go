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
	SelectCredential(ctx context.Context, orgID, provider, model string, strategy service.CredentialStrategy, excludeIDs map[string]bool) (*service.DecryptedCredential, error)
	SetCooldown(ctx context.Context, id string, model string, until time.Time) error
	GetMinCooldown(ctx context.Context, orgID, provider, model string) (time.Duration, string, error)
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
	return g.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (g *AIGateway) ChatWithOptions(ctx context.Context, messages []llm.Message, chatOpts llm.ChatOptions) (resp *llm.Response, err error) {
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
		resp, err = g.chatFallback(ctx, messages, chatOpts)
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

	// Harness Independence: split entries into eligible (not the excluded model)
	// and excluded (the model to avoid, e.g. the coder model for a Review call).
	// The excluded set is only used as a graceful last resort — see below.
	var eligibleEntries, excludedEntries []models.ComboEntry
	if opts.ExcludeModelID != "" {
		for _, entry := range entries {
			if entry.Model == opts.ExcludeModelID {
				excludedEntries = append(excludedEntries, entry)
			} else {
				eligibleEntries = append(eligibleEntries, entry)
			}
		}
	} else {
		eligibleEntries = entries
	}

	var failures []string

	attempt := func(candidateEntries []models.ComboEntry) (*llm.Response, error) {
		var attemptFailures []string
		var retried bool

		for {
			attemptFailures = nil
			for _, entry := range candidateEntries {
				currentEntry := entry
				lastEntry = &currentEntry
				excluded := map[string]bool{}
				for {
					var cred *service.DecryptedCredential
					cred, err = g.credentialPool.SelectCredential(ctx, opts.OrgID, entry.Provider, entry.Model, g.strategy, excluded)
					if errors.Is(err, service.ErrNoCredentialsAvailable) {
						attemptFailures = append(attemptFailures, fmt.Sprintf("%s/%s: no credentials", entry.Provider, entry.Model))
						break
					}
					if err != nil {
						attemptFailures = append(attemptFailures, fmt.Sprintf("%s/%s: %v", entry.Provider, entry.Model, err))
						break
					}
					lastCred = cred

					var provider llm.Provider
					provider, err = g.providerFactory(cred, entry.Model)
					if err != nil {
						attemptFailures = append(attemptFailures, fmt.Sprintf("%s/%s: %v", entry.Provider, entry.Model, err))
						break
					}
					attempts := 4
					for callAttempt := 0; callAttempt < attempts; callAttempt++ {
						resp, err = provider.ChatWithOptions(ctx, messages, chatOpts)
						if err == nil {
							break
						}
						if isTransientError(err) && callAttempt < attempts-1 {
							backoff := time.Duration(1000*(1<<callAttempt)) * time.Millisecond
							log.Printf("[AIGateway] Transient error calling %s/%s: %v. Retrying in %v (attempt %d/%d)...", entry.Provider, entry.Model, err, backoff, callAttempt+1, attempts)
							timer := time.NewTimer(backoff)
							select {
							case <-ctx.Done():
								timer.Stop()
								return nil, ctx.Err()
							case <-timer.C:
							}
							continue
						}
						break
					}
					if err != nil {
						attemptFailures = append(attemptFailures, fmt.Sprintf("%s/%s[%s]: %v", entry.Provider, entry.Model, cred.ID, err))
						if isTransientError(err) {
							_ = g.credentialPool.SetCooldown(ctx, cred.ID, entry.Model, time.Now().Add(g.cooldown))
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

			if !retried {
				var lowestCD time.Duration = -1
				var lowestCredID string
				for _, entry := range candidateEntries {
					cd, credID, cdErr := g.credentialPool.GetMinCooldown(ctx, opts.OrgID, entry.Provider, entry.Model)
					if cdErr == nil && cd >= 0 {
						if lowestCD == -1 || cd < lowestCD {
							lowestCD = cd
							lowestCredID = credID
						}
					}
				}

				if lowestCD > 0 && lowestCD < 30*time.Second {
					log.Printf("All credentials in cooldown. Waiting %ds for credential %s...", int(lowestCD.Seconds()), lowestCredID)
					timer := time.NewTimer(lowestCD)
					select {
					case <-ctx.Done():
						timer.Stop()
						return nil, ctx.Err()
					case <-timer.C:
					}
					retried = true
					continue
				}
			}
			break
		}
		failures = append(failures, attemptFailures...)
		return nil, fmt.Errorf("exhausted: %s", strings.Join(attemptFailures, "; "))
	}

	if len(eligibleEntries) > 0 {
		if resp, respErr := attempt(eligibleEntries); respErr == nil {
			return resp, nil
		} else if errors.Is(respErr, context.Canceled) || errors.Is(respErr, context.DeadlineExceeded) {
			err = respErr
			return nil, err
		}
	}

	// Graceful Harness Independence fallback: every eligible model failed (or
	// none existed because the excluded model was the only one configured for
	// this level group). Reuse the excluded model rather than blocking the task.
	if len(excludedEntries) > 0 {
		log.Printf("[AIGateway] Harness Independence fallback: forcing review using the original coder model (%s)", opts.ExcludeModelID)
		if resp, respErr := attempt(excludedEntries); respErr == nil {
			if trace := llm.RouteTraceFromCtx(ctx); trace != nil {
				trace.SelfReviewFallback = true
				trace.ExcludedModel = opts.ExcludeModelID
				trace.ActualModel = resp.Model
			}
			return resp, nil
		} else if errors.Is(respErr, context.Canceled) || errors.Is(respErr, context.DeadlineExceeded) {
			err = respErr
			return nil, err
		}
	}

	err = fmt.Errorf("ai gateway exhausted routes: %s", strings.Join(failures, "; "))
	return nil, err
}

func (g *AIGateway) chatFallback(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	if g.fallback == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	return g.fallback.ChatWithOptions(ctx, messages, opts)
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

// isTransientError delegates to the single canonical classifier (REQ-M05) — do not
// reintroduce a separate copy of this logic here.
func isTransientError(err error) bool {
	return llm.IsTransientError(err)
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
