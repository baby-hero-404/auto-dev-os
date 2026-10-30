package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/baby-hero-404/aicli/capture/ticket"
	"github.com/baby-hero-404/aicli/sandbox"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// credService serves one decrypted credential; every other method is unused.
type credService struct {
	ProviderCredentialService
	files map[string]string
	err   error
}

func (c credService) GetDecryptedCredential(_ context.Context, orgID, id string) (string, map[string]string, error) {
	if orgID != "org-1" || id != "cred-1" {
		return "", nil, errors.New("sql: no rows (internal detail)")
	}
	return "cli:claude", c.files, c.err
}

// shellRuntime plays the sandbox: it records what the shell would find in HOME, then exits.
type shellRuntime struct {
	sandbox.StubRuntime
	mu   sync.Mutex
	home map[string]string
}

func (s *shellRuntime) RunInteractive(_ context.Context, req sandbox.CommandRequest, _ io.Reader, _, _ io.Writer) error {
	seen := map[string]string{}
	_ = filepath.WalkDir(req.Workspace, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(req.Workspace, p)
			seen[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	s.mu.Lock()
	s.home = seen
	s.mu.Unlock()
	return nil
}

func terminalServer(t *testing.T, route func(chi.Router)) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	route(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func dialTerminal(srv *httptest.Server, path, origin string) (*websocket.Conn, *http.Response, error) {
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, hdr)
}

func TestCLITestTerminal_SeedsTheCredentialAndNeverReturnsIt(t *testing.T) {
	rt := &shellRuntime{}
	svc := credService{files: map[string]string{".claude/.credentials.json": "SECRET-TOKEN"}}
	h := NewCLITestHandler(rt, svc, []string{"http://app.test"})
	srv := terminalServer(t, func(r chi.Router) { r.Get("/organizations/{orgID}/cli-test/terminal", h.Terminal) })

	token, _ := h.term.tickets.Mint(context.Background(), ticket.Ticket{Subject: "u1", Scope: "org-1", Value: "cred-1"})
	conn, _, err := dialTerminal(srv, "/organizations/org-1/cli-test/terminal?ticket="+token, "http://app.test")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var frames []string
	var exit map[string]any
	for exit == nil {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			t.Fatalf("read frame: %v (frames %v)", err, frames)
		}
		b, _ := json.Marshal(f)
		frames = append(frames, string(b))
		if f["type"] == "exit" {
			exit = f
		}
	}
	rt.mu.Lock()
	home := rt.home
	rt.mu.Unlock()
	if home[".claude/.credentials.json"] != "SECRET-TOKEN" {
		t.Errorf("the shell did not find the credential in HOME: %v", home)
	}
	if _, has := exit["payload"]; has || strings.Contains(strings.Join(frames, "\n"), "SECRET-TOKEN") {
		t.Errorf("the credential went back to the browser: %v", frames)
	}
}

func TestCLITestTerminal_UnknownCredentialDoesNotLeakDetails(t *testing.T) {
	h := NewCLITestHandler(&shellRuntime{}, credService{}, []string{"http://app.test"})
	srv := terminalServer(t, func(r chi.Router) { r.Get("/organizations/{orgID}/cli-test/terminal", h.Terminal) })
	token, _ := h.term.tickets.Mint(context.Background(), ticket.Ticket{Subject: "u1", Scope: "org-1", Value: "someone-elses"})
	conn, _, err := dialTerminal(srv, "/organizations/org-1/cli-test/terminal?ticket="+token, "http://app.test")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			t.Fatalf("no error frame before the connection ended: %v", err)
		}
		if f["type"] != "error" {
			continue // the banner
		}
		if msg, _ := f["message"].(string); strings.Contains(msg, "sql") || !strings.Contains(msg, "credential") {
			t.Errorf("error frame = %v, want a generic credential error", f)
		}
		return
	}
}

// The terminal used to accept any Origin; a page on another site must not reach a sandbox.
func TestTerminals_RefuseForeignOrigins(t *testing.T) {
	auth := NewCLIAuthHandler(&shellRuntime{}, []string{"http://app.test"})
	test := NewCLITestHandler(&shellRuntime{}, credService{}, []string{"http://app.test"})
	srv := terminalServer(t, func(r chi.Router) {
		r.Get("/organizations/{orgID}/cli-auth/terminal", auth.Terminal)
		r.Get("/organizations/{orgID}/cli-test/terminal", test.Terminal)
	})
	for name, tc := range map[string]struct {
		path string
		term *terminal
	}{
		"cli-auth": {"/organizations/org-1/cli-auth/terminal", auth.term},
		"cli-test": {"/organizations/org-1/cli-test/terminal", test.term},
	} {
		t.Run(name, func(t *testing.T) {
			token, _ := tc.term.tickets.Mint(context.Background(), ticket.Ticket{Subject: "u1", Scope: "org-1", Value: "x"})
			_, resp, err := dialTerminal(srv, tc.path+"?ticket="+token, "https://evil.example")
			if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
				t.Fatalf("foreign origin got through: err=%v resp=%v", err, resp)
			}
		})
	}
}

func TestMintWSTicket_RequiresItsField(t *testing.T) {
	h := NewCLITestHandler(&shellRuntime{}, credService{}, nil)
	req := mintRequest(`{}`, "org-1", "org-1")
	rr := httptest.NewRecorder()
	h.MintWSTicket(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing credential_id: %d, want 400", rr.Code)
	}
}

func mintRequest(body, claimsOrg, pathOrg string) *http.Request {
	req := httptest.NewRequest("POST", "/organizations/"+pathOrg+"/cli-test/ws-ticket", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), authClaimsKey, &service.TokenClaims{Subject: "u1", OrgID: claimsOrg}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", pathOrg)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
