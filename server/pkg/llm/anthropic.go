package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
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
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Provider:          a.Name(),
		Model:             a.model,
		LevelGroup:        levelGroupForModel(a.model),
		InputCostPer1K:    inputCostPer1K(a.model),
		OutputCostPer1K:   outputCostPer1K(a.model),
		MaxContextTokens:  200000,
		MaxResponseTokens: 4096,
	}
}

// Chat sends messages to the Anthropic Messages API.
func (a *Anthropic) Chat(ctx context.Context, messages []Message) (*Response, error) {
	return a.ChatWithOptions(ctx, messages, ChatOptions{})
}

func (a *Anthropic) ChatWithOptions(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
	// Anthropic requires system message to be separate from the messages array.
	var systemPrompt string
	var userMessages []map[string]any

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			userMessages = append(userMessages, anthropicMessage(msg))
		}
	}

	payload := map[string]interface{}{
		"model":      a.model,
		"max_tokens": 4096,
		"messages":   userMessages,
	}
	if systemPrompt != "" {
		// System prompt is stable per job/step and often reused across the tool loop —
		// mark it cacheable so repeated calls in the same job hit Anthropic's prompt cache.
		payload["system"] = []map[string]any{
			{
				"type":          "text",
				"text":          systemPrompt,
				"cache_control": map[string]any{"type": "ephemeral"},
			},
		}
	}
	if len(opts.Tools) > 0 {
		tools := anthropicTools(opts.Tools)
		// Tool definitions don't change within a job; mark the last one so the whole
		// tools block (and everything before it) is cached as one prefix.
		tools[len(tools)-1]["cache_control"] = map[string]any{"type": "ephemeral"}
		payload["tools"] = tools
		if opts.ToolChoice != "" && opts.ToolChoice != "auto" {
			payload["tool_choice"] = map[string]any{"type": opts.ToolChoice}
		}
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

	var contentParts []string
	var toolCalls []ToolCall
	for _, part := range result.Content {
		if part.Type == "tool_use" {
			toolCalls = append(toolCalls, ToolCall{
				ID:        part.ID,
				Name:      part.Name,
				Arguments: string(part.Input),
			})
			continue
		}
		if part.Text != "" {
			contentParts = append(contentParts, part.Text)
		}
	}

	if result.Usage.CacheReadInputTokens > 0 || result.Usage.CacheCreationInputTokens > 0 {
		slog.Debug("anthropic prompt cache usage",
			"cache_read_tokens", result.Usage.CacheReadInputTokens,
			"cache_write_tokens", result.Usage.CacheCreationInputTokens,
			"input_tokens", result.Usage.InputTokens,
		)
	}

	return &Response{
		Content:          strings.Join(contentParts, "\n"),
		Model:            result.Model,
		PromptTokens:     result.Usage.InputTokens,
		OutputTokens:     result.Usage.OutputTokens,
		CacheWriteTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:  result.Usage.CacheReadInputTokens,
		ToolCalls:        toolCalls,
	}, nil
}

func anthropicMessage(msg Message) map[string]any {
	role := msg.Role
	if role == "tool" {
		return map[string]any{
			"role": "user",
			"content": []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     msg.Content,
			}},
		}
	}
	if len(msg.ToolCalls) > 0 {
		var content []map[string]any
		if msg.Content != "" {
			content = append(content, map[string]any{"type": "text", "text": msg.Content})
		}
		for _, call := range msg.ToolCalls {
			input := any(map[string]any{})
			if strings.TrimSpace(call.Arguments) != "" {
				var parsed any
				if err := json.Unmarshal([]byte(call.Arguments), &parsed); err == nil {
					input = parsed
				}
			}
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    call.ID,
				"name":  call.Name,
				"input": input,
			})
		}
		return map[string]any{"role": role, "content": content}
	}
	return map[string]any{"role": role, "content": msg.Content}
}

func anthropicTools(tools []ToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		var schema any = map[string]any{"type": "object"}
		if len(tool.Parameters) > 0 {
			_ = json.Unmarshal(tool.Parameters, &schema)
		}
		out = append(out, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": schema,
		})
	}
	return out
}

// Anthropic response structures.
type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}
