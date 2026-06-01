package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNineRouter_Constructor(t *testing.T) {
	nr := NewNineRouter("test-key", "test-model", "http://localhost:20128/v1/")
	if nr.baseURL != "http://localhost:20128/v1" {
		t.Errorf("expected trailing slash to be trimmed: got %q", nr.baseURL)
	}

	nrDefault := NewNineRouter("test-key", "test-model", "")
	if nrDefault.baseURL != "http://localhost:20128/v1" {
		t.Errorf("expected default base URL: got %q", nrDefault.baseURL)
	}

	if nr.Name() != "9router" {
		t.Errorf("expected Name to be '9router', got %q", nr.Name())
	}
}

func TestNineRouter_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected bearer token, got %q", r.Header.Get("Authorization"))
		}

		response := `{
			"model": "test-model",
			"choices": [
				{
					"message": {
						"content": "hello world from 9router mock"
					}
				}
			],
			"usage": {
				"prompt_tokens": 15,
				"completion_tokens": 8
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	nr := NewNineRouter("test-key", "test-model", server.URL)
	resp, err := nr.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if resp.Content != "hello world from 9router mock" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.PromptTokens != 15 || resp.OutputTokens != 8 {
		t.Errorf("unexpected usage prompt=%d output=%d", resp.PromptTokens, resp.OutputTokens)
	}
}
