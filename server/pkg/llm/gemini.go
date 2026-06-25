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

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// Gemini implements the Provider interface for the Google Gemini API.
type Gemini struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGemini creates a new Gemini provider.
func NewGemini(apiKey, model string) *Gemini {
	return &Gemini{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (g *Gemini) Name() string { return "gemini" }

func (g *Gemini) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Provider:          g.Name(),
		Model:             g.model,
		LevelGroup:        levelGroupForModel(g.model),
		InputCostPer1K:    inputCostPer1K(g.model),
		OutputCostPer1K:   outputCostPer1K(g.model),
		MaxContextTokens:  1000000,
		MaxResponseTokens: 4096,
	}
}

// Chat sends messages to the Google Gemini API.
func (g *Gemini) Chat(ctx context.Context, messages []Message) (*Response, error) {
	return g.ChatWithOptions(ctx, messages, ChatOptions{})
}

func (g *Gemini) ChatWithOptions(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
	// Convert our standard messages to Gemini's format.
	var systemInstruction string
	var contents []geminiContent

	for _, msg := range messages {
		if msg.Role == "system" {
			systemInstruction = msg.Content
			continue
		}

		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "tool" {
			contents = append(contents, geminiContent{
				Role: "function",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFunctionResponse{
						Name: msg.ToolName,
						Response: map[string]any{
							"content": msg.Content,
						},
					},
				}},
			})
			continue
		}
		parts := []geminiPart{}
		if msg.Content != "" {
			parts = append(parts, geminiPart{Text: msg.Content})
		}
		for _, call := range msg.ToolCalls {
			var args map[string]any
			if call.Arguments != "" {
				_ = json.Unmarshal([]byte(call.Arguments), &args)
			}
			parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{Name: call.Name, Args: args}})
		}
		if len(parts) == 0 {
			parts = []geminiPart{{Text: msg.Content}}
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: parts,
		})
	}

	payload := geminiRequest{
		Contents: contents,
	}
	if systemInstruction != "" {
		payload.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemInstruction}},
		}
	}
	if len(opts.Tools) > 0 {
		payload.Tools = geminiTools(opts.Tools)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(geminiAPIURL, g.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result geminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini returned no content")
	}

	var contentParts []string
	var toolCalls []ToolCall
	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			argsBytes, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, ToolCall{
				Name:      part.FunctionCall.Name,
				Arguments: string(argsBytes),
			})
			continue
		}
		if part.Text != "" {
			contentParts = append(contentParts, part.Text)
		}
	}

	content := strings.Join(contentParts, "\n")
	if content == "" {
		fmt.Printf("DEBUG: Gemini returned empty content. Raw response: %s\n", string(respBody))
	}

	return &Response{
		Content:      content,
		Model:        g.model,
		PromptTokens: result.UsageMetadata.PromptTokenCount,
		OutputTokens: result.UsageMetadata.CandidatesTokenCount,
		ToolCalls:    toolCalls,
	}, nil
}

func geminiTools(tools []ToolDefinition) []geminiTool {
	functions := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		var parameters map[string]any
		if len(tool.Parameters) > 0 {
			_ = json.Unmarshal(tool.Parameters, &parameters)
		}
		if parameters == nil {
			parameters = map[string]any{"type": "object"}
		}
		functions = append(functions, geminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
		})
	}
	return []geminiTool{{FunctionDeclarations: functions}}
}

// Gemini request/response structures.
type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Tools             []geminiTool    `json:"tools,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"function_declarations"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string         `json:"name"`
					Args map[string]any `json:"args"`
				} `json:"functionCall,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}
