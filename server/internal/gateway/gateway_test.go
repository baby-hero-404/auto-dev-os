package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakePool struct {
	credentials []*service.DecryptedCredential
	selected    []string
	cooldowns   []string
	minCooldown time.Duration
	minCredID   string
}

func (p *fakePool) SelectCredential(_ context.Context, orgID, provider, model string, strategy service.CredentialStrategy, excludeIDs map[string]bool) (*service.DecryptedCredential, error) {
	for _, cred := range p.credentials {
		if cred.Provider == provider && !excludeIDs[cred.ID] {
			p.selected = append(p.selected, cred.ID)
			return cred, nil
		}
	}
	return nil, service.ErrNoCredentialsAvailable
}

func (p *fakePool) SetCooldown(_ context.Context, id string, model string, _ time.Time) error {
	p.cooldowns = append(p.cooldowns, id+":"+model)
	return nil
}

func (p *fakePool) GetMinCooldown(_ context.Context, orgID, provider, model string) (time.Duration, string, error) {
	return p.minCooldown, p.minCredID, nil
}

type fakeResolver struct {
	models []models.ProviderModel
	err    error
}

func (r *fakeResolver) ResolveModels(_ context.Context, orgID, levelGroup string) ([]models.ProviderModel, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.models, nil
}

type fakeProvider struct {
	name string
	resp *llm.Response
	err  error
}

func (p *fakeProvider) Name() string {
	return p.name
}

func (p *fakeProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return p.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (p *fakeProvider) ChatWithOptions(_ context.Context, _ []llm.Message, _ llm.ChatOptions) (*llm.Response, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.resp, nil
}

func TestAIGatewayChatFallsBackWithoutOrg(t *testing.T) {
	fallback := &fakeProvider{name: "fallback", resp: &llm.Response{Content: "ok", Model: "fallback-model"}}
	g := NewAIGateway(Options{FallbackProvider: fallback})

	resp, err := g.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Content != "ok" || resp.Model != "fallback-model" {
		t.Fatalf("unexpected fallback response: %+v", resp)
	}
}

func TestAIGatewayChatRotatesCredentialOnRateLimit(t *testing.T) {
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-1", Provider: "openai", APIKey: "sk-1"},
		{ID: "cred-2", Provider: "openai", APIKey: "sk-2"},
	}}
	resolver := &fakeResolver{models: []models.ProviderModel{{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true}}}
	attempts := 0
	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		Cooldown:              time.Second,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			attempts++
			if cred.ID == "cred-1" {
				return &fakeProvider{name: "openai", err: HTTPStatusError{StatusCode: 429, Message: "rate limited"}}, nil
			}
			return &fakeProvider{name: "openai", resp: &llm.Response{Content: "ok", Model: "gpt-4o"}}, nil
		},
	})
	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1"})

	resp, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Content != "ok" || attempts != 2 {
		t.Fatalf("expected second credential success, resp=%+v attempts=%d", resp, attempts)
	}
	if len(pool.cooldowns) != 1 || pool.cooldowns[0] != "cred-1:gpt-4o" {
		t.Fatalf("expected first credential cooldown, got %v", pool.cooldowns)
	}
	if strings.Join(pool.selected, ",") != "cred-1,cred-2" {
		t.Fatalf("unexpected credential selection order: %v", pool.selected)
	}
}

func TestAIGatewayChatFallsBackToNextModel(t *testing.T) {
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-openai", Provider: "openai", APIKey: "sk-openai"},
		{ID: "cred-anthropic", Provider: "anthropic", APIKey: "sk-anthropic"},
	}}
	resolver := &fakeResolver{models: []models.ProviderModel{
		{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true},
		{Provider: "anthropic", ModelName: "claude-3-5-sonnet", Priority: 1, LevelGroup: "balanced", IsActive: true},
	}}

	calledGPT4o := false
	calledClaude := false

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			if model == "gpt-4o" {
				calledGPT4o = true
				return &fakeProvider{name: "openai", err: errors.New("openai API down")}, nil
			}
			if model == "claude-3-5-sonnet" {
				calledClaude = true
				return &fakeProvider{name: "anthropic", resp: &llm.Response{Content: "anthropic ok", Model: "claude-3-5-sonnet"}}, nil
			}
			return nil, errors.New("unexpected model")
		},
	})

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1"})
	resp, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if !calledGPT4o {
		t.Error("expected gpt-4o to be tried first")
	}
	if !calledClaude {
		t.Error("expected fallback to claude-3-5-sonnet")
	}
	if resp.Content != "anthropic ok" || resp.Model != "claude-3-5-sonnet" {
		t.Fatalf("expected response from fallback model, got: %+v", resp)
	}
}

// TestAIGatewayChatTelemetry_RecordsPerAttempt verifies that a call falling over from one
// provider/credential to another records one usage/cost row per attempt (Task 4.3 / REQ-M09),
// not just a single row for the last attempt of the whole ChatWithOptions call.
func TestAIGatewayChatTelemetry_RecordsPerAttempt(t *testing.T) {
	recorder := &fakeUsageRecorder{ch: make(chan llm.UsageRecord, 2)}
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-openai", Provider: "openai", APIKey: "sk-openai"},
		{ID: "cred-anthropic", Provider: "anthropic", APIKey: "sk-anthropic"},
	}}
	resolver := &fakeResolver{models: []models.ProviderModel{
		{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true},
		{Provider: "anthropic", ModelName: "claude-3-5-sonnet", Priority: 1, LevelGroup: "balanced", IsActive: true},
	}}

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		Recorder:              recorder,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			if model == "gpt-4o" {
				return &fakeProvider{name: "openai", err: errors.New("openai API down")}, nil
			}
			if model == "claude-3-5-sonnet" {
				return &fakeProvider{name: "anthropic", resp: &llm.Response{Content: "anthropic ok", Model: "claude-3-5-sonnet", PromptTokens: 3, OutputTokens: 4}}, nil
			}
			return nil, errors.New("unexpected model")
		},
	})

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1"})
	_, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	// Recording happens in a background goroutine per attempt, so the two records may arrive
	// in either order — key them by provider instead of asserting a fixed sequence.
	byProvider := map[string]llm.UsageRecord{}
	for i := 0; i < 2; i++ {
		select {
		case rec := <-recorder.ch:
			byProvider[rec.Provider] = rec
		case <-time.After(1 * time.Second):
			t.Fatalf("timed out waiting for telemetry record %d/2", i+1)
		}
	}

	openaiRec, ok := byProvider["openai"]
	if !ok || openaiRec.Status != "failed" || openaiRec.CredentialID != "cred-openai" {
		t.Errorf("expected a failed openai attempt record, got %+v (ok=%v)", openaiRec, ok)
	}
	anthropicRec, ok := byProvider["anthropic"]
	if !ok || anthropicRec.Status != "ok" || anthropicRec.CredentialID != "cred-anthropic" {
		t.Errorf("expected a successful anthropic attempt record, got %+v (ok=%v)", anthropicRec, ok)
	}
	if anthropicRec.PromptTokens != 3 || anthropicRec.OutputTokens != 4 {
		t.Errorf("expected successful attempt's token usage recorded, got %+v", anthropicRec)
	}
}

func TestIsRateLimitErrorSupportsTypedStatusAndStringFallback(t *testing.T) {
	if !isTransientError(HTTPStatusError{StatusCode: 402}) {
		t.Fatalf("expected typed 402 to be rate limit")
	}
	if !isTransientError(errors.New("provider quota exceeded")) {
		t.Fatalf("expected quota string to be rate limit")
	}
	// REQ-M05: network-level errors are now classified as transient at the gateway layer
	// too (previously only the outer llmrunner layer recognized these, so the credential
	// never got cooled down at the point of failure). See pkg/llm/transient_error.go.
	if !isTransientError(errors.New("connection refused")) {
		t.Fatalf("expected generic connection error to now be classified as transient (REQ-M05)")
	}
	if !isTransientError(errors.New("context deadline exceeded")) {
		t.Fatalf("expected timeout error to be classified as transient")
	}
	if !isTransientError(errors.New("unexpected EOF")) {
		t.Fatalf("expected EOF error to be classified as transient")
	}
	if isTransientError(errors.New("invalid api key")) {
		t.Fatalf("did not expect a non-transient auth error to be classified as transient")
	}
}

type fakeUsageRecorder struct {
	ch      chan llm.UsageRecord
	records []llm.UsageRecord
}

func (f *fakeUsageRecorder) RecordLLMUsage(ctx context.Context, record llm.UsageRecord) error {
	f.records = append(f.records, record)
	if f.ch != nil {
		f.ch <- record
	}
	return nil
}

func TestAIGatewayChatTelemetry(t *testing.T) {
	// 1. Success case with org ID
	recorder := &fakeUsageRecorder{ch: make(chan llm.UsageRecord, 1)}
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-1", Provider: "openai", APIKey: "sk-1"},
	}}
	resolver := &fakeResolver{models: []models.ProviderModel{{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true}}}

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		Recorder:              recorder,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{
				Content:      "ok",
				Model:        "gpt-4o",
				PromptTokens: 10,
				OutputTokens: 20,
			}}, nil
		},
	})

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{
		OrgID:     "org-1",
		ProjectID: "project-1",
		AgentID:   "agent-1",
		TaskID:    "task-1",
	})

	_, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	select {
	case rec := <-recorder.ch:
		if rec.OrgID != "org-1" || rec.ProjectID != "project-1" || rec.AgentID != "agent-1" || rec.TaskID != "task-1" {
			t.Errorf("incorrect metadata: %+v", rec)
		}
		if rec.Status != "ok" {
			t.Errorf("expected status ok, got %s", rec.Status)
		}
		if rec.PromptTokens != 10 || rec.OutputTokens != 20 {
			t.Errorf("incorrect tokens: prompt=%d output=%d", rec.PromptTokens, rec.OutputTokens)
		}
		if rec.CredentialID != "cred-1" {
			t.Errorf("expected credential-1, got %s", rec.CredentialID)
		}
		if rec.LevelGroup != "balanced" {
			t.Errorf("expected level group balanced, got %s", rec.LevelGroup)
		}
		if rec.Model != "gpt-4o" {
			t.Errorf("expected model gpt-4o, got %s", rec.Model)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for telemetry record")
	}

	// 2. Failure case with org ID
	recorder = &fakeUsageRecorder{ch: make(chan llm.UsageRecord, 1)}
	g = NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		Recorder:              recorder,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", err: errors.New("anthropic API error")}, nil
		},
	})

	_, err = g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err == nil {
		t.Fatal("expected error from Chat, got nil")
	}

	select {
	case rec := <-recorder.ch:
		if rec.Status != "failed" {
			t.Errorf("expected status failed, got %s", rec.Status)
		}
		if rec.Error == "" {
			t.Error("expected non-empty error field")
		}
		if rec.CredentialID != "cred-1" {
			t.Errorf("expected credential-1, got %s", rec.CredentialID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for telemetry record")
	}

	// 3. Without org ID
	recorder = &fakeUsageRecorder{ch: make(chan llm.UsageRecord, 1)}
	fallback := &fakeProvider{name: "fallback", resp: &llm.Response{Content: "ok", Model: "fallback-model"}}
	g = NewAIGateway(Options{
		FallbackProvider: fallback,
		Recorder:         recorder,
	})

	_, err = g.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	select {
	case <-recorder.ch:
		t.Error("telemetry recorded when org ID was empty")
	case <-time.After(100 * time.Millisecond):
		// Success: no telemetry sent
	}
}

func TestAIGatewayRouteEntriesUsesResolverOnly(t *testing.T) {
	pool := &fakePool{}
	resolver := &fakeResolver{models: []models.ProviderModel{{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true}}}
	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
	})

	entries, err := g.routeEntries(context.Background(), llm.RouteOptions{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("routeEntries returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if len(pool.selected) != 0 {
		t.Fatalf("expected routeEntries to avoid credential selection, got %v", pool.selected)
	}
}

func TestAIGatewayChatEmptyResolverReturnsError(t *testing.T) {
	pool := &fakePool{}
	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: &fakeResolver{models: nil},
	})

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1"})
	_, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err == nil {
		t.Fatal("expected error when no active models are configured")
	}
	if !strings.Contains(err.Error(), "no active models configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAIGatewayChatTelemetry_ContextTimeout(t *testing.T) {
	recorder := &fakeUsageRecorder{ch: make(chan llm.UsageRecord, 1)}
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-1", Provider: "openai", APIKey: "sk-1"},
	}}
	resolver := &fakeResolver{models: []models.ProviderModel{{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true}}}

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		Recorder:              recorder,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{
				Content:      "ok",
				Model:        "gpt-4o",
				PromptTokens: 5,
				OutputTokens: 5,
			}}, nil
		},
	})

	mainCtx, cancelMain := context.WithCancel(context.Background())
	ctx := llm.WithRouteOptions(mainCtx, llm.RouteOptions{
		OrgID:     "org-1",
		ProjectID: "project-1",
	})

	_, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	// Cancel the parent context immediately
	cancelMain()

	// Telemetry should still succeed and write to recorder because it uses context.WithoutCancel
	select {
	case rec := <-recorder.ch:
		if rec.OrgID != "org-1" {
			t.Errorf("expected org-1, got %s", rec.OrgID)
		}
		if rec.Status != "ok" {
			t.Errorf("expected status ok, got %s", rec.Status)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for telemetry record despite parent context cancel")
	}
}

func TestAIGatewayWaitAndRetry(t *testing.T) {
	// Scenario 1: cooldown < 30s (e.g., 50ms), success on retry.
	pool := &fakePool{
		credentials: []*service.DecryptedCredential{}, // initially empty to trigger error
		minCooldown: 50 * time.Millisecond,
		minCredID:   "cred-1",
	}
	resolver := &fakeResolver{models: []models.ProviderModel{{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true}}}

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{Content: "retry success", Model: "gpt-4o"}}, nil
		},
	})

	// Run a goroutine to populate credentials after 20ms (simulating cooldown expiration)
	go func() {
		time.Sleep(20 * time.Millisecond)
		pool.credentials = []*service.DecryptedCredential{
			{ID: "cred-1", Provider: "openai", APIKey: "sk-1"},
		}
	}()

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1"})
	start := time.Now()
	resp, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if resp.Content != "retry success" {
		t.Errorf("expected Content 'retry success', got %q", resp.Content)
	}
	if duration < 50*time.Millisecond {
		t.Errorf("expected wait of at least 50ms, got %v", duration)
	}

	// Scenario 2: cooldown >= 30s (e.g., 30s), fails immediately without waiting.
	pool2 := &fakePool{
		credentials: []*service.DecryptedCredential{},
		minCooldown: 30 * time.Second,
		minCredID:   "cred-2",
	}
	g2 := NewAIGateway(Options{
		CredentialPool:        pool2,
		ProviderModelResolver: resolver,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{Content: "retry success", Model: "gpt-4o"}}, nil
		},
	})
	start = time.Now()
	_, err = g2.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	duration = time.Since(start)
	if err == nil {
		t.Fatal("expected error immediately, got nil")
	}
	if duration >= 1*time.Second {
		t.Errorf("expected immediate failure without 30s wait, took %v", duration)
	}

	// Scenario 3: Context canceled during wait.
	pool3 := &fakePool{
		credentials: []*service.DecryptedCredential{},
		minCooldown: 10 * time.Second,
		minCredID:   "cred-3",
	}
	g3 := NewAIGateway(Options{
		CredentialPool:        pool3,
		ProviderModelResolver: resolver,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{Content: "retry success", Model: "gpt-4o"}}, nil
		},
	})
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancelCtx = llm.WithRouteOptions(cancelCtx, llm.RouteOptions{OrgID: "org-1"})
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start = time.Now()
	_, err = g3.Chat(cancelCtx, []llm.Message{{Role: "user", Content: "hello"}})
	duration = time.Since(start)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if duration >= 1*time.Second {
		t.Errorf("expected quick cancellation, took %v", duration)
	}
}

func TestAIGatewayChatWithOptions_HarnessIndependenceExcludesModel(t *testing.T) {
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-openai", Provider: "openai", APIKey: "sk-openai"},
		{ID: "cred-anthropic", Provider: "anthropic", APIKey: "sk-anthropic"},
	}}
	resolver := &fakeResolver{models: []models.ProviderModel{
		{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true},
		{Provider: "anthropic", ModelName: "claude-3-5-sonnet", Priority: 1, LevelGroup: "balanced", IsActive: true},
	}}

	calledGPT4o := false
	calledClaude := false

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			if model == "gpt-4o" {
				calledGPT4o = true
				return &fakeProvider{name: "openai", resp: &llm.Response{Content: "coder model", Model: "gpt-4o"}}, nil
			}
			if model == "claude-3-5-sonnet" {
				calledClaude = true
				return &fakeProvider{name: "anthropic", resp: &llm.Response{Content: "reviewer model", Model: "claude-3-5-sonnet"}}, nil
			}
			return nil, errors.New("unexpected model")
		},
	})

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1", ExcludeModelID: "gpt-4o"})
	ctx, trace := llm.WithRouteTrace(ctx)
	resp, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "review this"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if calledGPT4o {
		t.Error("excluded model gpt-4o should never have been called")
	}
	if !calledClaude {
		t.Error("expected the non-excluded model claude-3-5-sonnet to be used")
	}
	if resp.Model != "claude-3-5-sonnet" {
		t.Fatalf("expected response from claude-3-5-sonnet, got: %+v", resp)
	}
	if trace.SelfReviewFallback {
		t.Error("SelfReviewFallback should be false when an alternative model was available and used")
	}
}

func TestAIGatewayChatWithOptions_HarnessIndependenceGracefulFallback(t *testing.T) {
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-openai", Provider: "openai", APIKey: "sk-openai"},
	}}
	// Only one model configured in this level group, and it's the one we want to exclude.
	resolver := &fakeResolver{models: []models.ProviderModel{
		{Provider: "openai", ModelName: "gpt-4o", Priority: 0, LevelGroup: "balanced", IsActive: true},
	}}

	g := NewAIGateway(Options{
		CredentialPool:        pool,
		ProviderModelResolver: resolver,
		ProviderFactory: func(cred *service.DecryptedCredential, model string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{Content: "fallback response", Model: "gpt-4o"}}, nil
		},
	})

	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1", ExcludeModelID: "gpt-4o"})
	ctx, trace := llm.WithRouteTrace(ctx)
	resp, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "review this"}})
	if err != nil {
		t.Fatalf("expected graceful fallback to succeed, got error: %v", err)
	}
	if resp.Content != "fallback response" {
		t.Fatalf("expected fallback response, got: %+v", resp)
	}
	if !trace.SelfReviewFallback {
		t.Error("expected SelfReviewFallback to be true when no alternative model exists")
	}
	if trace.ExcludedModel != "gpt-4o" {
		t.Errorf("expected ExcludedModel to be gpt-4o, got %q", trace.ExcludedModel)
	}
	if trace.ActualModel != "gpt-4o" {
		t.Errorf("expected ActualModel to be gpt-4o (reused), got %q", trace.ActualModel)
	}
}
