package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/config"
)

type contextKey string

const routeOptionsKey contextKey = "llm_route_options"

// Model tiers used by the gateway router.
const (
	TierFast     = "fast"
	TierBalanced = "balanced"
	TierPowerful = "powerful"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // message text
}

// Response represents the LLM's response.
type Response struct {
	Content      string `json:"content"`       // generated text
	Model        string `json:"model"`         // model used
	PromptTokens int    `json:"prompt_tokens"` // input tokens consumed
	OutputTokens int    `json:"output_tokens"` // output tokens generated
}

// ProviderMetadata exposes normalized routing and cost metadata.
type ProviderMetadata struct {
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
	Tier              string  `json:"tier"`
	InputCostPer1K    float64 `json:"input_cost_per_1k"`
	OutputCostPer1K   float64 `json:"output_cost_per_1k"`
	MaxContextTokens  int     `json:"max_context_tokens"`
	MaxResponseTokens int     `json:"max_response_tokens"`
}

// MetadataProvider can be implemented by providers that expose model metadata.
type MetadataProvider interface {
	Metadata() ProviderMetadata
}

// RouteOptions carries per-request gateway routing and budget hints.
type RouteOptions struct {
	Complexity      string  `json:"complexity,omitempty"`
	ProjectID       string  `json:"project_id,omitempty"`
	AgentID         string  `json:"agent_id,omitempty"`
	TaskID          string  `json:"task_id,omitempty"`
	MaxInputTokens  int     `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int     `json:"max_output_tokens,omitempty"`
	MaxCostUSD      float64 `json:"max_cost_usd,omitempty"`
}

// WithRouteOptions annotates a request for gateway routing.
func WithRouteOptions(ctx context.Context, opts RouteOptions) context.Context {
	opts.Complexity = strings.ToLower(opts.Complexity)
	return context.WithValue(ctx, routeOptionsKey, opts)
}

// RouteOptionsFromContext returns gateway routing metadata from context.
func RouteOptionsFromContext(ctx context.Context) (RouteOptions, bool) {
	opts, ok := ctx.Value(routeOptionsKey).(RouteOptions)
	return opts, ok
}

// Provider is the interface that all LLM backends must implement.
type Provider interface {
	// Chat sends a list of messages and returns the model's response.
	Chat(ctx context.Context, messages []Message) (*Response, error)

	// Name returns the provider identifier (e.g. "openai").
	Name() string
}

// NewProvider creates the appropriate LLM provider based on configuration.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.LLM.Provider {
	case "openai":
		return NewOpenAI(cfg.LLM.APIKey, cfg.LLM.Model), nil
	case "anthropic":
		return NewAnthropic(cfg.LLM.APIKey, cfg.LLM.Model), nil
	case "gemini":
		return NewGemini(cfg.LLM.APIKey, cfg.LLM.Model), nil
	case "9router":
		return NewNineRouter(cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.BaseURL), nil
	case "gateway":
		return NewGatewayFromConfig(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.LLM.Provider)
	}
}
