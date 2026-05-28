package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// Anthropic implements the Provider interface for the Anthropic Messages API.
type Anthropic struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropic creates a new Anthropic provider.
func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Provider:          a.Name(),
		Model:             a.model,
		Tier:              tierForModel(a.model),
		InputCostPer1K:    inputCostPer1K(a.model),
		OutputCostPer1K:   outputCostPer1K(a.model),
		MaxContextTokens:  200000,
		MaxResponseTokens: 4096,
	}
}

// Chat sends messages to the Anthropic Messages API.
func (a *Anthropic) Chat(ctx context.Context, messages []Message) (*Response, error) {
	// Anthropic requires system message to be separate from the messages array.
	var systemPrompt string
	var userMessages []Message

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			userMessages = append(userMessages, msg)
		}
	}

	payload := map[string]interface{}{
		"model":      a.model,
		"max_tokens": 4096,
		"messages":   userMessages,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("Anthropic returned no content")
	}

	return &Response{
		Content:      result.Content[0].Text,
		Model:        result.Model,
		PromptTokens: result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}, nil
}

// Anthropic response structures.
type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
