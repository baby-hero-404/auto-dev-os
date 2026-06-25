package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAI implements the Provider interface for the OpenAI API.
type OpenAI struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAI creates a new OpenAI provider.
func NewOpenAI(apiKey, model string) *OpenAI {
	return &OpenAI{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Provider:          o.Name(),
		Model:             o.model,
		LevelGroup:        levelGroupForModel(o.model),
		InputCostPer1K:    inputCostPer1K(o.model),
		OutputCostPer1K:   outputCostPer1K(o.model),
		MaxContextTokens:  128000,
		MaxResponseTokens: 4096,
	}
}

// Chat sends messages to the OpenAI Chat Completions API.
func (o *OpenAI) Chat(ctx context.Context, messages []Message) (*Response, error) {
	return o.ChatWithOptions(ctx, messages, ChatOptions{})
}

func (o *OpenAI) ChatWithOptions(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
	payload := openAIChatPayload(o.model, messages, opts)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parseOpenAIChatResponse(respBody)
}

func openAIChatPayload(model string, messages []Message, opts ChatOptions) map[string]interface{} {
	payload := map[string]interface{}{
		"model":    model,
		"messages": openAIMessages(messages),
	}
	if len(opts.Tools) > 0 {
		payload["tools"] = openAITools(opts.Tools)
		if opts.ToolChoice != "" {
			payload["tool_choice"] = opts.ToolChoice
		}
	}
	return payload
}

func openAIMessages(messages []Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{
			"role": msg.Role,
		}
		if msg.Role == "tool" {
			m["tool_call_id"] = msg.ToolCallID
			m["content"] = msg.Content
		} else {
			m["content"] = msg.Content
			if len(msg.ToolCalls) > 0 {
				var calls []map[string]any
				for _, call := range msg.ToolCalls {
					calls = append(calls, map[string]any{
						"id":   call.ID,
						"type": "function",
						"function": map[string]any{
							"name":      call.Name,
							"arguments": call.Arguments,
						},
					})
				}
				m["tool_calls"] = calls
			}
		}
		out = append(out, m)
	}
	return out
}

func openAITools(tools []ToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		var parameters any
		if len(tool.Parameters) > 0 {
			if err := json.Unmarshal(tool.Parameters, &parameters); err != nil {
				parameters = map[string]any{"type": "object"}
			}
		} else {
			parameters = map[string]any{"type": "object"}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  parameters,
			},
		})
	}
	return out
}

func parseOpenAIChatResponse(respBody []byte) (*Response, error) {
	var result openaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	msg := result.Choices[0].Message
	resp := &Response{
		Content:      msg.Content,
		Model:        result.Model,
		PromptTokens: result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}
	for _, toolCall := range msg.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: toolCall.Function.Arguments,
		})
	}
	return resp, nil
}

// OpenAI response structures.
type openaiResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}
