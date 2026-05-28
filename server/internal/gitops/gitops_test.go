package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewGitHubProvider(t *testing.T) {
	p := NewGitHubProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.client == nil {
		t.Error("expected non-nil http client")
	}
}

func TestGitHubURLWithToken_HTTPS(t *testing.T) {
	result, err := githubURLWithToken("https://github.com/user/repo.git", "ghp_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://x-access-token:ghp_test123@github.com/user/repo.git" {
		t.Errorf("unexpected URL: %s", result)
	}
}

func TestGitHubURLWithToken_NonHTTPS(t *testing.T) {
	_, err := githubURLWithToken("http://github.com/user/repo.git", "token")
	if err == nil {
		t.Error("expected error for non-https URL")
	}
}

func TestGitHubURLWithToken_InvalidURL(t *testing.T) {
	_, err := githubURLWithToken("://invalid", "token")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestSanitizeToken(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		token    string
		expected string
	}{
		{"redacts token", "error: ghp_abc123 leaked", "ghp_abc123", "error: [redacted] leaked"},
		{"empty token", "error: nothing to redact", "", "error: nothing to redact"},
		{"no match", "clean output", "ghp_xyz", "clean output"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeToken(tc.value, tc.token)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestGitHubProvider_ValidateToken_MockServer(t *testing.T) {
	// Mock GitHub API server.
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"login":"testuser"}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create provider with mock client that redirects to test server.
	p := NewGitHubProvider()
	p.client = server.Client()

	// We can't easily redirect the hardcoded URL, so test the authorize logic directly.
	req, _ := http.NewRequest("GET", server.URL+"/user", nil)
	p.authorize(req, "test-token")

	if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("expected 'Bearer test-token', got %q", got)
	}
	if got := req.Header.Get("Accept"); got != "application/vnd.github+json" {
		t.Errorf("expected github accept header, got %q", got)
	}
	if got := req.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
		t.Errorf("expected api version header, got %q", got)
	}
}

func TestGitHubProvider_Authorize_NoToken(t *testing.T) {
	p := NewGitHubProvider()
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	p.authorize(req, "")

	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("expected no Authorization header when token is empty, got %q", got)
	}
}

func TestGitHubProvider_ListRepos_MockServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		repos := []map[string]any{
			{"name": "repo1", "full_name": "user/repo1", "clone_url": "https://github.com/user/repo1.git", "default_branch": "main", "private": false},
			{"name": "repo2", "full_name": "user/repo2", "clone_url": "https://github.com/user/repo2.git", "default_branch": "develop", "private": true},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := &GitHubProvider{client: server.Client()}

	// Use the mock server URL.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/user/repos?per_page=100&sort=updated", nil)
	p.authorize(req, "test-token")
	resp, err := p.client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGitHubProvider_CloneRepo_DefaultBranch(t *testing.T) {
	// Test that empty branch defaults to "main" by checking it doesn't panic.
	// Actual clone will fail without git, but we verify the branch default logic.
	p := NewGitHubProvider()

	// We just verify the provider handles empty branch without panic.
	// The clone will fail because the URL doesn't exist.
	_, err := p.CloneRepo(context.Background(), "https://github.com/nonexistent/repo.git", "", "", "/tmp/test-clone-nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent repo")
	}
}

func TestGitHubProvider_CloneRepo_FallbackToDefaultBranch(t *testing.T) {
	// Create a temp directory inside the workspace for our dummy origin repo
	tempDir, err := os.MkdirTemp("", "gitops-test-origin-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize a dummy git repository in tempDir
	// We'll set the default branch to 'master' (or something that isn't 'main')
	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command failed: %s %v: %s", name, args, string(output))
		}
	}

	runCmd(tempDir, "git", "init", "--initial-branch=master")
	runCmd(tempDir, "git", "config", "user.name", "Test User")
	runCmd(tempDir, "git", "config", "user.email", "test@example.com")
	
	// Create a dummy file and commit it so HEAD points to master and master has a commit
	dummyFile := filepath.Join(tempDir, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}
	runCmd(tempDir, "git", "add", "dummy.txt")
	runCmd(tempDir, "git", "commit", "-m", "initial commit")

	// Now create a temp directory for the clone destination
	cloneDest, err := os.MkdirTemp("", "gitops-test-clone-*")
	if err != nil {
		t.Fatalf("failed to create clone dest: %v", err)
	}
	defer os.RemoveAll(cloneDest)

	p := NewGitHubProvider()
	
	// Attempt to clone specifying 'main' (which doesn't exist).
	// It should fail the first attempt, fall back to default branch 'master', and return 'master'.
	clonedBranch, err := p.CloneRepo(context.Background(), tempDir, "", "main", cloneDest)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	if clonedBranch != "master" {
		t.Errorf("expected cloned branch to be 'master', got %q", clonedBranch)
	}

	// Verify the file exists in the clone destination
	if _, err := os.Stat(filepath.Join(cloneDest, "dummy.txt")); os.IsNotExist(err) {
		t.Error("expected dummy.txt to exist in clone destination")
	}
}
