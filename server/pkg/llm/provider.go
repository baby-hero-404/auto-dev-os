package llm

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/config"
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

// Provider is the interface that all LLM backends must implement.
type Provider interface {
	// Chat sends a list of messages and returns the model's response.
	Chat(ctx context.Context, messages []Message) (*Response, error)

	// Name returns the provider identifier (e.g. "openai").
	Name() string
}

// NewProvider creates the appropriate LLM provider based on configuration.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.LLMProvider {
	case "openai":
		return NewOpenAI(cfg.APIKey, cfg.LLMModel), nil
	case "anthropic":
		return NewAnthropic(cfg.APIKey, cfg.LLMModel), nil
	case "gemini":
		return NewGemini(cfg.APIKey, cfg.LLMModel), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.LLMProvider)
	}
}
