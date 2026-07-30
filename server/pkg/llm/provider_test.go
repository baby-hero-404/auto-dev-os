package llm

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeUsageRecorder struct {
	calls int
}

func (f *fakeUsageRecorder) RecordLLMUsage(ctx context.Context, usage UsageRecord) error {
	f.calls++
	return nil
}

// TestNewProviderWithRecorder_ThreadsRecorderIntoGatewayFallback guards
// against the recorder being silently dropped when building a Provider for
// cfg.LLM.Provider=="gateway" — it used to be hardcoded nil in NewProvider
// regardless of what the caller had available (AIGateway.chatFallback's
// telemetry gap).
func TestNewProviderWithRecorder_ThreadsRecorderIntoGatewayFallback(t *testing.T) {
	recorder := &fakeUsageRecorder{}
	cfg := &config.Config{LLM: config.LLMConfig{
		Provider:     "gateway",
		OpenAIAPIKey: "sk-test",
	}}
	provider, err := NewProviderWithRecorder(cfg, recorder)
	if err != nil {
		t.Fatalf("NewProviderWithRecorder returned error: %v", err)
	}
	gw, ok := provider.(*Gateway)
	if !ok {
		t.Fatalf("expected *Gateway, got %T", provider)
	}
	if gw.recorder != recorder {
		t.Fatal("expected the recorder passed to NewProviderWithRecorder to be threaded into the built Gateway")
	}
}

func TestNewProvider_NilRecorderIsFineForNonGatewayProviders(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{Provider: models.ProviderOpenAI, APIKey: "sk-test"}}
	if _, err := NewProvider(cfg); err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}
}
