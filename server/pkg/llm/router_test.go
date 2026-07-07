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
	opts  ChatOptions
}

func (p *fakeProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	return p.ChatWithOptions(ctx, messages, ChatOptions{})
}

func (p *fakeProvider) ChatWithOptions(_ context.Context, _ []Message, opts ChatOptions) (*Response, error) {
	p.calls++
	p.opts = opts
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

func TestGateway_HarnessIndependence(t *testing.T) {
	coderModel := &fakeProvider{
		name: "primary",
		meta: ProviderMetadata{Provider: "primary", Model: "coder-model", LevelGroup: LevelBalanced},
		resp: &Response{Content: "coder output", Model: "coder-model", PromptTokens: 10, OutputTokens: 5},
	}
	reviewerModel := &fakeProvider{
		name: "fallback",
		meta: ProviderMetadata{Provider: "fallback", Model: "reviewer-model", LevelGroup: LevelBalanced},
		resp: &Response{Content: "reviewer output", Model: "reviewer-model", PromptTokens: 10, OutputTokens: 5},
	}

	gateway, err := NewGateway([]FallbackChain{
		newFallbackChain(LevelBalanced, []Provider{coderModel, reviewerModel}),
	}, GatewayOptions{DefaultLevelGroup: LevelBalanced})
	if err != nil {
		t.Fatalf("NewGateway returned error: %v", err)
	}

	ctx := WithRouteOptions(context.Background(), RouteOptions{Complexity: "medium", ExcludeModelID: "coder-model"})
	resp, err := gateway.Chat(ctx, []Message{{Role: "user", Content: "review this"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Model != "reviewer-model" {
		t.Fatalf("Expected harness independence to skip coder-model and use reviewer-model, got %s", resp.Model)
	}
	if coderModel.calls != 0 || reviewerModel.calls != 1 {
		t.Fatalf("unexpected call counts coder=%d reviewer=%d", coderModel.calls, reviewerModel.calls)
	}
}

func TestGateway_HarnessIndependenceGracefulFallback(t *testing.T) {
	onlyModel := &fakeProvider{
		name: "primary",
		meta: ProviderMetadata{Provider: "primary", Model: "only-model", LevelGroup: LevelBalanced},
		resp: &Response{Content: "fallback output", Model: "only-model", PromptTokens: 10, OutputTokens: 5},
	}

	gateway, err := NewGateway([]FallbackChain{
		newFallbackChain(LevelBalanced, []Provider{onlyModel}),
	}, GatewayOptions{DefaultLevelGroup: LevelBalanced})
	if err != nil {
		t.Fatalf("NewGateway returned error: %v", err)
	}

	ctx := WithRouteOptions(context.Background(), RouteOptions{Complexity: "medium", ExcludeModelID: "only-model"})
	resp, err := gateway.Chat(ctx, []Message{{Role: "user", Content: "review this"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Model != "only-model" {
		t.Fatalf("Expected graceful fallback to use only-model despite exclusion, got %s", resp.Model)
	}
	if onlyModel.calls != 1 {
		t.Fatalf("unexpected call counts onlyModel=%d", onlyModel.calls)
	}
}
