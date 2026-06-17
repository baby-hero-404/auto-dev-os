package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		Tier:              tierForModel(g.model),
		InputCostPer1K:    inputCostPer1K(g.model),
		OutputCostPer1K:   outputCostPer1K(g.model),
		MaxContextTokens:  1000000,
		MaxResponseTokens: 4096,
	}
}

// Chat sends messages to the Google Gemini API.
func (g *Gemini) Chat(ctx context.Context, messages []Message) (*Response, error) {
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
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
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

	part := result.Candidates[0].Content.Parts[0]
	if part.FunctionCall != nil {
		return nil, fmt.Errorf("Gemini attempted to make a function call: %s", part.FunctionCall.Name)
	}

	content := part.Text
	if content == "" {
		fmt.Printf("DEBUG: Gemini returned empty content. Raw response: %s\n", string(respBody))
	}

	return &Response{
		Content:      content,
		Model:        g.model,
		PromptTokens: result.UsageMetadata.PromptTokenCount,
		OutputTokens: result.UsageMetadata.CandidatesTokenCount,
	}, nil
}

// Gemini request/response structures.
type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string `json:"name"`
				} `json:"functionCall,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}
