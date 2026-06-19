package llm

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name  string
	meta  ProviderMetadata
	resp  *Response
	err   error
	calls int
}

func (p *fakeProvider) Chat(context.Context, []Message) (*Response, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.resp, nil
}

func (p *fakeProvider) Name() string {
	return p.name
}

func (p *fakeProvider) Metadata() ProviderMetadata {
	return p.meta
}

func TestGateway_RoutesByComplexityAndFallsBack(t *testing.T) {
	primary := &fakeProvider{
		name: "primary",
		meta: ProviderMetadata{Provider: "primary", Model: "fast-a", LevelGroup: LevelFast},
		err:  errors.New("temporary outage"),
	}
	fallback := &fakeProvider{
		name: "fallback",
		meta: ProviderMetadata{Provider: "fallback", Model: "fast-b", LevelGroup: LevelFast},
		resp: &Response{Content: "ok", Model: "fast-b", PromptTokens: 10, OutputTokens: 5},
	}
	powerful := &fakeProvider{
		name: "powerful",
		meta: ProviderMetadata{Provider: "powerful", Model: "powerful-a", LevelGroup: LevelPowerful},
		resp: &Response{Content: "hard", Model: "powerful-a", PromptTokens: 10, OutputTokens: 5},
	}

	gateway, err := NewGateway([]FallbackChain{
		newFallbackChain(LevelFast, []Provider{primary, fallback}),
		newFallbackChain(LevelPowerful, []Provider{powerful}),
	}, GatewayOptions{DefaultLevelGroup: LevelFast})
	if err != nil {
		t.Fatalf("NewGateway returned error: %v", err)
	}

	ctx := WithRouteOptions(context.Background(), RouteOptions{Complexity: "easy"})
	resp, err := gateway.Chat(ctx, []Message{{Role: "user", Content: "small task"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected fallback response, got %q", resp.Content)
	}
	if primary.calls != 1 || fallback.calls != 1 || powerful.calls != 0 {
		t.Fatalf("unexpected call counts primary=%d fallback=%d powerful=%d", primary.calls, fallback.calls, powerful.calls)
	}
}

func TestGateway_CircuitBreakerBlocksLargePrompt(t *testing.T) {
	provider := &fakeProvider{
		name: "provider",
		meta: ProviderMetadata{Provider: "provider", Model: "m", LevelGroup: LevelBalanced},
		resp: &Response{Content: "ok"},
	}
	gateway, err := NewGateway([]FallbackChain{
		newFallbackChain(LevelBalanced, []Provider{provider}),
	}, GatewayOptions{DefaultLevelGroup: LevelBalanced, MaxTokensPerCall: 1})
	if err != nil {
		t.Fatalf("NewGateway returned error: %v", err)
	}

	_, err = gateway.Chat(context.Background(), []Message{{Role: "user", Content: "this exceeds one estimated token"}})
	if err == nil || !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected circuit breaker error, got %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called after preflight block")
	}
}
