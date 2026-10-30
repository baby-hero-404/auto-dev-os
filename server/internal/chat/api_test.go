package chat_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/chat"
	"github.com/auto-code-os/auto-code-os/server/internal/handler"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/baby-hero-404/aicli/sandbox"
	"github.com/baby-hero-404/aicli/sandbox/sandboxmock"
)

const agyReply = `{"event":"init","conversation_id":"cli-session-1","init":{}}
{"event":"step_update","step_update":{"conversation_id":"cli-session-1","step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"Hello from the assistant.\n"}}
{"event":"result","result":{"conversation_id":"cli-session-1","status":"SUCCESS","usage":{"input_tokens":3,"output_tokens":5,"total_tokens":8}}}
`

// tokens the fake auth service accepts, and who they are
var identities = map[string]*service.TokenClaims{
	"admin":     {Subject: "u1", OrgID: "org-1", Role: models.UserRoleAdmin, Type: "access"},
	"member":    {Subject: "u2", OrgID: "org-1", Role: "member", Type: "access"},
	"other-org": {Subject: "u3", OrgID: "org-2", Role: models.UserRoleAdmin, Type: "access"},
}

type fakeAuth struct{ handler.AuthService }

func (fakeAuth) VerifyToken(token, _ string) (*service.TokenClaims, error) {
	if c, ok := identities[token]; ok {
		return c, nil
	}
	return nil, errors.New("bad token")
}

// credStore is the organization's credential store with one agy credential.
type credStore struct {
	mu      sync.Mutex
	payload map[string]string
	updates []map[string]string
}

func (c *credStore) ListByOrg(context.Context, string) ([]models.ProviderCredentialResponse, error) {
	return []models.ProviderCredentialResponse{
		{ID: "cred-other", Provider: "cli:claude", Status: models.ProviderCredentialStatusActive, Configured: true},
		{ID: "cred-1", Provider: "cli:antigravity", Status: models.ProviderCredentialStatusActive, Configured: true},
	}, nil
}

func (c *credStore) GetDecryptedCredential(_ context.Context, _, id string) (string, map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{}
	for k, v := range c.payload {
		out[k] = v
	}
	return "cli:antigravity", out, nil
}

func (c *credStore) UpdateCredentialPayload(_ context.Context, _, _ string, payload map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, payload)
	return nil
}

type rig struct {
	srv   *httptest.Server
	rt    *sandboxmock.Runtime
	creds *credStore
}

func newRig(t *testing.T, results ...*sandbox.CommandResult) *rig {
	t.Helper()
	rt := &sandboxmock.Runtime{Results: results}
	creds := &credStore{payload: map[string]string{".gemini/oauth_creds.json": "old-token", ".gemini/google_accounts.json": "acct"}}
	svc, err := chat.New(chat.Config{Profile: "antigravity_notools", DataDir: t.TempDir()}, chat.OpenTestDB(t), creds, rt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.Close(ctx)
	})
	router := handler.NewRouter(handler.Deps{AuthSvc: fakeAuth{}, Chat: svc, CORSAllowedOrigins: "http://app.test"})
	// a real server enforces WriteTimeout; recorders do not
	srv := httptest.NewUnstartedServer(router)
	srv.Config.WriteTimeout = 300 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)
	return &rig{srv: srv, rt: rt, creds: creds}
}

func (r *rig) call(t *testing.T, method, token, path, body string, hdr ...string) (int, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), method, r.srv.URL+"/api/v1/organizations/org-1/chat"+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var b strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		b.WriteString(sc.Text())
	}
	return resp.StatusCode, b.String()
}

func TestChat_AccessRules(t *testing.T) {
	r := newRig(t, &sandbox.CommandResult{Stdout: agyReply})
	for name, tc := range map[string]struct {
		token string
		want  int
	}{
		"no token":                  {"", http.StatusUnauthorized},
		"a member (not an admin)":   {"member", http.StatusForbidden},
		"admin of another org":      {"other-org", http.StatusForbidden},
		"admin of this org (lists)": {"admin", http.StatusOK},
	} {
		if got, body := r.call(t, http.MethodGet, tc.token, "/conversations", ""); got != tc.want {
			t.Errorf("%s: %d %s, want %d", name, got, body, tc.want)
		}
	}
	// a page on another site cannot reach it even with a valid token
	if got, _ := r.call(t, http.MethodPost, "admin", "/conversations", `{"profile":"antigravity_notools"}`, "Origin", "https://evil.example"); got != http.StatusForbidden {
		t.Errorf("foreign origin: %d, want 403", got)
	}
	// the profile allowlist holds
	if got, _ := r.call(t, http.MethodPost, "admin", "/conversations", `{"profile":"claude_code"}`); got != http.StatusBadRequest {
		t.Errorf("profile outside the allowlist: %d, want 400", got)
	}
}

func TestChat_ConversationsAreScopedToTheirOrgAndUser(t *testing.T) {
	r := newRig(t, &sandbox.CommandResult{Stdout: agyReply})
	_, body := r.call(t, http.MethodPost, "admin", "/conversations", `{"profile":"antigravity_notools"}`)
	var created struct{ ID string }
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.ID == "" {
		t.Fatalf("create: %s", body)
	}
	identities["admin2"] = &service.TokenClaims{Subject: "u9", OrgID: "org-1", Role: models.UserRoleAdmin, Type: "access"}
	t.Cleanup(func() { delete(identities, "admin2") })
	// another admin of the same organization still does not see it
	if got, _ := r.call(t, http.MethodGet, "admin2", "/conversations/"+created.ID, ""); got != http.StatusNotFound {
		t.Errorf("another user's conversation: %d, want 404", got)
	}
	if _, list := r.call(t, http.MethodGet, "admin2", "/conversations", ""); strings.Contains(list, created.ID) {
		t.Errorf("the conversation leaked into another user's list: %s", list)
	}
}

// The whole flow through the router's real middleware: the event stream outlives the server's write
// timeout and the router's 30 s request timeout, the sandbox gets the organization's credential, and
// a token the CLI refreshes is merged back.
func TestChat_StreamAndCredentialRoundTrip(t *testing.T) {
	refreshed := &sandbox.CommandResult{
		Stdout:                 agyReply,
		UpdatedCredentialFiles: map[string]string{sandbox.HomeDir + "/.gemini/oauth_creds.json": "new-token"},
	}
	r := newRig(t, refreshed)
	_, body := r.call(t, http.MethodPost, "admin", "/conversations", `{"profile":"antigravity_notools"}`)
	var created struct{ ID string }
	_ = json.Unmarshal([]byte(body), &created)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, r.srv.URL+"/api/v1/organizations/org-1/chat/conversations/"+created.ID+"/stream", nil)
	req.Header.Set("Authorization", "Bearer admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("stream: %v %v", resp, err)
	}
	defer resp.Body.Close()
	kinds := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if k, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				kinds <- k
			}
		}
		close(kinds)
	}()

	time.Sleep(700 * time.Millisecond) // past the server's write timeout, with nothing to say yet
	if got, b := r.call(t, http.MethodPost, "admin", "/conversations/"+created.ID+"/messages", `{"message":"hi","clientMessageId":"m1"}`); got != http.StatusAccepted {
		t.Fatalf("send: %d %s", got, b)
	}
	timeout := time.After(5 * time.Second)
	for done := false; !done; {
		select {
		case k, ok := <-kinds:
			if !ok {
				t.Fatal("the stream was cut before the turn completed")
			}
			done = k == "turn.completed"
		case <-timeout:
			t.Fatal("no turn.completed on the stream")
		}
	}

	reqs := r.rt.Requests()
	if len(reqs) != 1 {
		t.Fatalf("sandbox runs = %d", len(reqs))
	}
	if got := reqs[0].CredentialFiles; len(got) != 2 || got[sandbox.HomeDir+"/.gemini/oauth_creds.json"] != "old-token" {
		t.Errorf("sandbox credentials = %v, want the organization's antigravity credential", got)
	}
	// the refreshed token is merged onto the stored files (the untouched one stays)
	r.creds.mu.Lock()
	defer r.creds.mu.Unlock()
	if len(r.creds.updates) != 1 || r.creds.updates[0][".gemini/oauth_creds.json"] != "new-token" || r.creds.updates[0][".gemini/google_accounts.json"] != "acct" {
		t.Errorf("credential updates = %v", r.creds.updates)
	}
}
