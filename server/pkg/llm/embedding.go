package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openAIEmbeddingsURL = "https://api.openai.com/v1/embeddings"

// Embedder generates dense vector embeddings for memory retrieval.
type Embedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
	Name() string
}

type OpenAIEmbedder struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *OpenAIEmbedder) Name() string {
	return "openai"
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, input string) ([]float32, error) {
	payload := map[string]any{
		"model": e.model,
		"input": input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEmbeddingsURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI embeddings error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("OpenAI embeddings returned no vectors")
	}
	return result.Data[0].Embedding, nil
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}
