package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/config"
)

type contextKey string

const routeOptionsKey contextKey = "llm_route_options"
const excludeModelIDKey contextKey = "llm_exclude_model_id"

// Model level groups used by the gateway router.
const (
	LevelFast     = "fast"
	LevelBalanced = "balanced"
	LevelPowerful = "powerful"
)

// Message represents a single message in a conversation.
type Message struct {
	Role          string     `json:"role"`                     // "system", "user", "assistant", "tool"
	Content       string     `json:"content"`                  // message text
	ToolCallID    string     `json:"tool_call_id,omitempty"`   // provider-specific tool call ID
	ToolName      string     `json:"tool_name,omitempty"`      // tool name for tool messages
	ToolArguments string     `json:"tool_arguments,omitempty"` // JSON arguments emitted by assistant
	ToolCalls     []ToolCall `json:"tool_calls,omitempty"`     // assistant-requested tool calls
}

// Response represents the LLM's response.
type Response struct {
	Content      string     `json:"content"`       // generated text
	Model        string     `json:"model"`         // model used
	PromptTokens int        `json:"prompt_tokens"` // input tokens consumed
	OutputTokens int        `json:"output_tokens"` // output tokens generated
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
}

// ToolDefinition defines a native LLM tool schema.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name"`
	Arguments        string `json:"arguments,omitempty"`
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// ChatOptions contains native tool definitions and choice constraints for the ChatWithOptions call.
type ChatOptions struct {
	Tools      []ToolDefinition `json:"tools,omitempty"`
	ToolChoice string           `json:"tool_choice,omitempty"`
}

// ProviderMetadata exposes normalized routing and cost metadata.
type ProviderMetadata struct {
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
	LevelGroup        string  `json:"level_group"`
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
	Complexity string `json:"complexity,omitempty"`
	OrgID      string `json:"org_id,omitempty"`
	ProjectID  string `json:"project_id,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	TaskID     string `json:"task_id,omitempty"`

	RouteName       string  `json:"route_name,omitempty"`
	MaxInputTokens  int     `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int     `json:"max_output_tokens,omitempty"`
	MaxCostUSD      float64 `json:"max_cost_usd,omitempty"`
	ExcludeModelID  string  `json:"exclude_model_id,omitempty"`
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

// WithExcludeModelID sets the exclude model ID in context for harness independence.
func WithExcludeModelID(ctx context.Context, excludeID string) context.Context {
	return context.WithValue(ctx, excludeModelIDKey, excludeID)
}

// ExcludeModelIDFromContext retrieves the excluded model ID from context.
func ExcludeModelIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(excludeModelIDKey).(string); ok {
		return val
	}
	return ""
}

// Provider is the interface that all LLM backends must implement.
type Provider interface {
	// Chat sends a list of messages and returns the model's response.
	Chat(ctx context.Context, messages []Message) (*Response, error)

	// ChatWithOptions sends messages with optional native tool schemas.
	ChatWithOptions(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error)

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
