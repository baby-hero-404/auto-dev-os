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
}

func (p *fakePool) SelectCredential(_ context.Context, orgID, provider string, strategy service.CredentialStrategy, excludeIDs map[string]bool) (*service.DecryptedCredential, error) {
	for _, cred := range p.credentials {
		if cred.Provider == provider && !excludeIDs[cred.ID] {
			p.selected = append(p.selected, cred.ID)
			return cred, nil
		}
	}
	return nil, service.ErrNoCredentialsAvailable
}

func (p *fakePool) SetCooldown(_ context.Context, id string, _ time.Time) error {
	p.cooldowns = append(p.cooldowns, id)
	return nil
}

type fakeVirtualKeys struct {
	key         *models.VirtualKey
	validateIn  string
	recordedID  string
	recorded    float64
	validateErr error
	recordErr   error
}

func (v *fakeVirtualKeys) Validate(_ context.Context, rawKey string, estimatedCost float64) (*models.VirtualKey, error) {
	v.validateIn = rawKey
	if v.validateErr != nil {
		return nil, v.validateErr
	}
	return v.key, nil
}

func (v *fakeVirtualKeys) RecordUsage(_ context.Context, id string, costUSD float64) error {
	v.recordedID = id
	v.recorded = costUSD
	return v.recordErr
}

type fakeRoutes struct {
	entries []models.ComboEntry
	err     error
}

func (r *fakeRoutes) ResolveRoute(_ context.Context, orgID, routeName, complexity string) (*service.ResolvedRoute, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &service.ResolvedRoute{Entries: r.entries}, nil
}

type fakeProvider struct {
	name string
	resp *llm.Response
	err  error
}

func (p *fakeProvider) Name() string {
	return p.name
}

func (p *fakeProvider) Chat(context.Context, []llm.Message) (*llm.Response, error) {
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

func TestAIGatewayChatRecordsVirtualKeyUsage(t *testing.T) {
	pool := &fakePool{credentials: []*service.DecryptedCredential{{ID: "cred-1", Provider: "openai", APIKey: "sk-test"}}}
	virtualKeys := &fakeVirtualKeys{key: &models.VirtualKey{ID: "vk-1"}}
	routes := &fakeRoutes{entries: []models.ComboEntry{{Provider: "openai", Model: "gpt-4o", Priority: 0}}}
	g := NewAIGateway(Options{
		CredentialPool:    pool,
		VirtualKeyService: virtualKeys,
		ModelRouteService: routes,
		ProviderFactory: func(*service.DecryptedCredential, string) (llm.Provider, error) {
			return &fakeProvider{name: "openai", resp: &llm.Response{Content: "ok", PromptTokens: 100, OutputTokens: 20}}, nil
		},
	})
	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1", VirtualKey: "sk-aco-test"})

	resp, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Model != "gpt-4o" {
		t.Fatalf("expected gateway to fill response model, got %q", resp.Model)
	}
	if virtualKeys.validateIn != "sk-aco-test" {
		t.Fatalf("virtual key was not validated")
	}
	if virtualKeys.recordedID != "vk-1" || virtualKeys.recorded <= 0 {
		t.Fatalf("usage not recorded: id=%q cost=%f", virtualKeys.recordedID, virtualKeys.recorded)
	}
}

func TestAIGatewayChatRotatesCredentialOnRateLimit(t *testing.T) {
	pool := &fakePool{credentials: []*service.DecryptedCredential{
		{ID: "cred-1", Provider: "openai", APIKey: "sk-1"},
		{ID: "cred-2", Provider: "openai", APIKey: "sk-2"},
	}}
	routes := &fakeRoutes{entries: []models.ComboEntry{{Provider: "openai", Model: "gpt-4o", Priority: 0}}}
	attempts := 0
	g := NewAIGateway(Options{
		CredentialPool:    pool,
		ModelRouteService: routes,
		Cooldown:          time.Second,
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
	if len(pool.cooldowns) != 1 || pool.cooldowns[0] != "cred-1" {
		t.Fatalf("expected first credential cooldown, got %v", pool.cooldowns)
	}
	if strings.Join(pool.selected, ",") != "cred-1,cred-2" {
		t.Fatalf("unexpected credential selection order: %v", pool.selected)
	}
}

func TestAIGatewayChatReturnsExhaustedRoutesWithVirtualKey(t *testing.T) {
	pool := &fakePool{}
	virtualKeys := &fakeVirtualKeys{key: &models.VirtualKey{ID: "vk-1"}}
	routes := &fakeRoutes{entries: []models.ComboEntry{{Provider: "openai", Model: "gpt-4o", Priority: 0}}}
	g := NewAIGateway(Options{CredentialPool: pool, VirtualKeyService: virtualKeys, ModelRouteService: routes})
	ctx := llm.WithRouteOptions(context.Background(), llm.RouteOptions{OrgID: "org-1", VirtualKey: "sk-aco-test"})

	_, err := g.Chat(ctx, []llm.Message{{Role: "user", Content: "hello"}})
	if err == nil || !strings.Contains(err.Error(), "no credentials") {
		t.Fatalf("expected no credentials error, got %v", err)
	}
}

func TestIsRateLimitErrorSupportsTypedStatusAndStringFallback(t *testing.T) {
	if !isTransientError(HTTPStatusError{StatusCode: 402}) {
		t.Fatalf("expected typed 402 to be rate limit")
	}
	if !isTransientError(errors.New("provider quota exceeded")) {
		t.Fatalf("expected quota string to be rate limit")
	}
	if isTransientError(errors.New("connection refused")) {
		t.Fatalf("did not expect generic connection error to be rate limit")
	}
}
