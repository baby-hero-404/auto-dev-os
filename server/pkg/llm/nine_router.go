package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// NineRouter implements the Provider interface for a generic OpenAI-compatible gateway like 9router.
type NineRouter struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewNineRouter creates a new generic OpenAI/9router provider.
func NewNineRouter(apiKey, model, baseURL string) *NineRouter {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:20128/v1"
	}
	return &NineRouter{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (n *NineRouter) Name() string { return "9router" }

func (n *NineRouter) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Provider:          n.Name(),
		Model:             n.model,
		Tier:              TierBalanced,
		InputCostPer1K:    0.0015,
		OutputCostPer1K:   0.002,
		MaxContextTokens:  128000,
		MaxResponseTokens: 4096,
	}
}

// Chat sends messages to the OpenAI-compatible completions endpoint.
func (n *NineRouter) Chat(ctx context.Context, messages []Message) (*Response, error) {
	payload := map[string]interface{}{
		"model":    n.model,
		"messages": messages,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := n.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+n.apiKey)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("9router/Gateway API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result openaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("gateway returned no choices")
	}

	return &Response{
		Content:      result.Choices[0].Message.Content,
		Model:        result.Model,
		PromptTokens: result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}, nil
}
