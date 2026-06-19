package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"go.opentelemetry.io/otel"
)

var ErrCircuitOpen = errors.New("llm gateway circuit breaker open")

type UsageRecord struct {
	ProjectID    string
	AgentID      string
	TaskID       string
	OrgID        string
	CredentialID string
	Provider     string
	Model        string
	LevelGroup   string
	PromptTokens int
	OutputTokens int
	CostUSD      float64
	LatencyMS    int64
	Status       string
	Error        string
}

type UsageRecorder interface {
	RecordLLMUsage(ctx context.Context, usage UsageRecord) error
}

type GatewayOptions struct {
	DefaultLevelGroup  string
	MaxTokensPerCall   int
	MaxCostUSDPerCall  float64
	DefaultOutputLimit int
	MaxRetries         int
	Recorder           UsageRecorder
}

type Gateway struct {
	chains             map[string]FallbackChain
	defaultLevelGroup  string
	maxTokensPerCall   int
	maxCostUSDPerCall  float64
	defaultOutputLimit int
	maxRetries         int
	recorder           UsageRecorder
}

func NewGateway(chains []FallbackChain, opts GatewayOptions) (*Gateway, error) {
	if len(chains) == 0 {
		return nil, fmt.Errorf("gateway requires at least one fallback chain")
	}
	g := &Gateway{
		chains:             map[string]FallbackChain{},
		defaultLevelGroup:  opts.DefaultLevelGroup,
		maxTokensPerCall:   opts.MaxTokensPerCall,
		maxCostUSDPerCall:  opts.MaxCostUSDPerCall,
		defaultOutputLimit: opts.DefaultOutputLimit,
		maxRetries:         opts.MaxRetries,
		recorder:           opts.Recorder,
	}
	if g.defaultLevelGroup == "" {
		g.defaultLevelGroup = LevelBalanced
	}
	if g.defaultOutputLimit == 0 {
		g.defaultOutputLimit = 2048
	}
	for _, chain := range chains {
		if chain.LevelGroup == "" {
			return nil, fmt.Errorf("fallback chain level group is required")
		}
		if len(chain.Providers) == 0 {
			return nil, fmt.Errorf("fallback chain %q has no providers", chain.LevelGroup)
		}
		g.chains[chain.LevelGroup] = chain
	}
	return g, nil
}

func NewGatewayFromConfig(cfg *config.Config) (*Gateway, error) {
	return NewGatewayFromConfigWithRecorder(cfg, nil)
}

func NewGatewayFromConfigWithRecorder(cfg *config.Config, recorder UsageRecorder) (*Gateway, error) {
	chains := []FallbackChain{}
	if cfg.LLM.OpenAIAPIKey != "" {
		chains = append(chains,
			newFallbackChain(LevelFast, []Provider{NewOpenAI(cfg.LLM.OpenAIAPIKey, cfg.LLM.FastModel)}),
			newFallbackChain(LevelBalanced, []Provider{NewOpenAI(cfg.LLM.OpenAIAPIKey, cfg.LLM.BalancedModel)}),
			newFallbackChain(LevelPowerful, []Provider{NewOpenAI(cfg.LLM.OpenAIAPIKey, cfg.LLM.PowerfulModel)}),
		)
	}
	if cfg.LLM.AnthropicAPIKey != "" {
		appendProvider(&chains, LevelBalanced, NewAnthropic(cfg.LLM.AnthropicAPIKey, cfg.LLM.AnthropicBalancedModel))
		appendProvider(&chains, LevelPowerful, NewAnthropic(cfg.LLM.AnthropicAPIKey, cfg.LLM.AnthropicPowerfulModel))
	}
	if cfg.LLM.GeminiAPIKey != "" {
		appendProvider(&chains, LevelFast, NewGemini(cfg.LLM.GeminiAPIKey, cfg.LLM.GeminiFastModel))
		appendProvider(&chains, LevelBalanced, NewGemini(cfg.LLM.GeminiAPIKey, cfg.LLM.GeminiBalancedModel))
	}
	return NewGateway(chains, GatewayOptions{
		DefaultLevelGroup:  LevelBalanced,
		MaxTokensPerCall:   cfg.LLM.CircuitMaxTokens,
		MaxCostUSDPerCall:  cfg.LLM.CircuitMaxCostUSD,
		DefaultOutputLimit: cfg.LLM.DefaultOutputTokens,
		MaxRetries:         cfg.LLM.MaxRetries,
		Recorder:           recorder,
	})
}

func appendProvider(chains *[]FallbackChain, levelGroup string, provider Provider) {
	for i := range *chains {
		if (*chains)[i].LevelGroup == levelGroup {
			(*chains)[i].Providers = append((*chains)[i].Providers, provider)
			return
		}
	}
	*chains = append(*chains, newFallbackChain(levelGroup, []Provider{provider}))
}

func (g *Gateway) Name() string { return "gateway" }

func (g *Gateway) Chat(ctx context.Context, messages []Message) (*Response, error) {
	ctx, span := otel.Tracer("auto-code-os/llm").Start(ctx, "llm.gateway.chat")
	defer span.End()
	opts, _ := RouteOptionsFromContext(ctx)
	levelGroup := opts.RouteName
	if levelGroup == "" || levelGroup == "default" {
		levelGroup = g.levelGroupForComplexity(opts.Complexity)
	}
	chain, ok := g.chains[levelGroup]
	if !ok {
		chain = g.chains[g.defaultLevelGroup]
	}
	if len(chain.Providers) == 0 {
		return nil, fmt.Errorf("no providers configured for level group %q", levelGroup)
	}

	inputTokens := estimateMessageTokens(messages)
	outputLimit := opts.MaxOutputTokens
	if outputLimit == 0 {
		outputLimit = g.defaultOutputLimit
	}
	if err := g.checkBudget(inputTokens, outputLimit, opts, metadataForProvider(chain.Providers[0])); err != nil {
		return nil, err
	}

	var failures []string
	for _, provider := range chain.Providers {
		meta := metadataForProvider(provider)
		resp, latency, err := g.chatWithRetry(ctx, provider, messages)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s/%s: %v", provider.Name(), meta.Model, err))
			g.record(ctx, opts, meta, nil, latency, "failed", err.Error())
			continue
		}
		if resp.Model == "" {
			resp.Model = meta.Model
		}
		cost := estimateCost(resp.PromptTokens, resp.OutputTokens, meta)
		if err := g.checkActualUsage(resp, cost, opts); err != nil {
			g.record(ctx, opts, meta, resp, latency, "blocked", err.Error())
			return nil, err
		}
		g.record(ctx, opts, meta, resp, latency, "ok", "")
		return resp, nil
	}

	return nil, fmt.Errorf("llm gateway exhausted fallbacks for level group %q: %s", levelGroup, strings.Join(failures, "; "))
}

func (g *Gateway) chatWithRetry(ctx context.Context, provider Provider, messages []Message) (*Response, int64, error) {
	attempts := g.maxRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	start := time.Now()
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := provider.Chat(ctx, messages)
		if err == nil {
			return resp, time.Since(start).Milliseconds(), nil
		}
		lastErr = err
		if attempt == attempts-1 {
			break
		}
		backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, time.Since(start).Milliseconds(), ctx.Err()
		case <-timer.C:
		}
	}
	return nil, time.Since(start).Milliseconds(), lastErr
}

func (g *Gateway) levelGroupForComplexity(complexity string) string {
	switch strings.ToLower(complexity) {
	case models.TaskComplexityEasy:
		return LevelFast
	case models.TaskComplexityHard:
		return LevelPowerful
	case models.TaskComplexityMedium:
		return LevelBalanced
	default:
		return g.defaultLevelGroup
	}
}

func (g *Gateway) checkBudget(inputTokens, outputLimit int, opts RouteOptions, meta ProviderMetadata) error {
	maxTokens := g.maxTokensPerCall
	if opts.MaxInputTokens > 0 {
		maxTokens = opts.MaxInputTokens
	}
	if maxTokens > 0 && inputTokens > maxTokens {
		return fmt.Errorf("%w: estimated input tokens %d exceed limit %d", ErrCircuitOpen, inputTokens, maxTokens)
	}
	maxCost := g.maxCostUSDPerCall
	if opts.MaxCostUSD > 0 {
		maxCost = opts.MaxCostUSD
	}
	if maxCost > 0 {
		cost := estimateCost(inputTokens, outputLimit, meta)
		if cost > maxCost {
			return fmt.Errorf("%w: estimated cost %.6f exceeds limit %.6f", ErrCircuitOpen, cost, maxCost)
		}
	}
	return nil
}

func (g *Gateway) checkActualUsage(resp *Response, cost float64, opts RouteOptions) error {
	maxTokens := g.maxTokensPerCall
	if opts.MaxInputTokens > 0 {
		maxTokens = opts.MaxInputTokens
	}
	if maxTokens > 0 && resp.PromptTokens > maxTokens {
		return fmt.Errorf("%w: prompt tokens %d exceed limit %d", ErrCircuitOpen, resp.PromptTokens, maxTokens)
	}
	if opts.MaxOutputTokens > 0 && resp.OutputTokens > opts.MaxOutputTokens {
		return fmt.Errorf("%w: output tokens %d exceed limit %d", ErrCircuitOpen, resp.OutputTokens, opts.MaxOutputTokens)
	}
	maxCost := g.maxCostUSDPerCall
	if opts.MaxCostUSD > 0 {
		maxCost = opts.MaxCostUSD
	}
	if maxCost > 0 && cost > maxCost {
		return fmt.Errorf("%w: actual cost %.6f exceeds limit %.6f", ErrCircuitOpen, cost, maxCost)
	}
	return nil
}

func (g *Gateway) record(ctx context.Context, opts RouteOptions, meta ProviderMetadata, resp *Response, latency int64, status, msg string) {
	if g.recorder == nil {
		return
	}
	usage := UsageRecord{
		ProjectID:  opts.ProjectID,
		AgentID:    opts.AgentID,
		TaskID:     opts.TaskID,
		Provider:   meta.Provider,
		Model:      meta.Model,
		LevelGroup: meta.LevelGroup,
		LatencyMS:  latency,
		Status:     status,
		Error:      msg,
	}
	if resp != nil {
		usage.PromptTokens = resp.PromptTokens
		usage.OutputTokens = resp.OutputTokens
		usage.CostUSD = estimateCost(resp.PromptTokens, resp.OutputTokens, meta)
	}
	_ = g.recorder.RecordLLMUsage(ctx, usage)
}

func metadataForProvider(provider Provider) ProviderMetadata {
	if p, ok := provider.(MetadataProvider); ok {
		return p.Metadata()
	}
	return ProviderMetadata{
		Provider:          provider.Name(),
		Model:             provider.Name(),
		LevelGroup:        LevelBalanced,
		InputCostPer1K:    0.005,
		OutputCostPer1K:   0.015,
		MaxResponseTokens: 4096,
	}
}

func estimateMessageTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Role) + len(msg.Content)
	}
	if total == 0 {
		return 0
	}
	return total/4 + len(messages)*4
}

func estimateCost(promptTokens, outputTokens int, meta ProviderMetadata) float64 {
	return (float64(promptTokens) / 1000 * meta.InputCostPer1K) +
		(float64(outputTokens) / 1000 * meta.OutputCostPer1K)
}
