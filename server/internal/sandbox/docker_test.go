package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCredentialFilesOverlap(t *testing.T) {
	cases := []struct {
		name            string
		authDirTarget   string
		credentialFiles map[string]string
		want            bool
	}{
		{
			name:          "credential file nested inside authDirs directory",
			authDirTarget: "/home/agent/.gemini",
			credentialFiles: map[string]string{
				"/home/agent/.gemini/antigravity-cli/antigravity-oauth-token": "token",
			},
			want: true,
		},
		{
			name:          "exact path match",
			authDirTarget: "/home/agent/.claude/.credentials.json",
			credentialFiles: map[string]string{
				"/home/agent/.claude/.credentials.json": "creds",
			},
			want: true,
		},
		{
			name:          "unrelated sibling path is not a false-positive prefix match",
			authDirTarget: "/home/agent/.gemini",
			credentialFiles: map[string]string{
				"/home/agent/.geminiextra/oauth-token": "token",
			},
			want: false,
		},
		{
			name:          "no overlap",
			authDirTarget: "/home/agent/.codex",
			credentialFiles: map[string]string{
				"/home/agent/.gemini/antigravity-cli/antigravity-oauth-token": "token",
			},
			want: false,
		},
		{
			name:            "no credential files at all",
			authDirTarget:   "/home/agent/.gemini",
			credentialFiles: nil,
			want:            false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := credentialFilesOverlap(tc.authDirTarget, tc.credentialFiles)
			if got != tc.want {
				t.Errorf("credentialFilesOverlap(%q, %v) = %v, want %v", tc.authDirTarget, tc.credentialFiles, got, tc.want)
			}
		})
	}
}

func TestSessionMountsOverlap(t *testing.T) {
	cases := []struct {
		name          string
		target        string
		sessionMounts map[string]string
		want          bool
	}{
		{
			name:   "target is exactly the session mount",
			target: "/home/agent/.claude",
			sessionMounts: map[string]string{
				"/home/agent/.claude": "/host/session/.claude",
			},
			want: true,
		},
		{
			name:   "target is nested inside session mount",
			target: "/home/agent/.claude/.credentials.json",
			sessionMounts: map[string]string{
				"/home/agent/.claude": "/host/session/.claude",
			},
			want: true,
		},
		{
			name:   "target is outside session mount",
			target: "/home/agent/.config",
			sessionMounts: map[string]string{
				"/home/agent/.claude": "/host/session/.claude",
			},
			want: false,
		},
		{
			name:   "unrelated sibling path is not a false-positive prefix match",
			target: "/home/agent/.claudedata",
			sessionMounts: map[string]string{
				"/home/agent/.claude": "/host/session/.claude",
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sessionMountsOverlap(tc.target, tc.sessionMounts)
			if got != tc.want {
				t.Errorf("sessionMountsOverlap(%q, %v) = %v, want %v", tc.target, tc.sessionMounts, got, tc.want)
			}
		})
	}
}

func TestCopyDirTree(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "antigravity-cli"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "antigravity-cli", "antigravity-oauth-token"), []byte("secret-token"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("antigravity-oauth-token", filepath.Join(src, "antigravity-cli", "token-link")); err != nil {
		t.Fatal(err)
	}

	if err := copyDirTree(src, dst); err != nil {
		t.Fatalf("copyDirTree: %v", err)
	}

	tokenPath := filepath.Join(dst, "antigravity-cli", "antigravity-oauth-token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read copied token: %v", err)
	}
	if string(data) != "secret-token" {
		t.Errorf("copied token = %q, want %q", data, "secret-token")
	}

	linkTarget, err := os.Readlink(filepath.Join(dst, "antigravity-cli", "token-link"))
	if err != nil {
		t.Fatalf("read copied symlink: %v", err)
	}
	if linkTarget != "antigravity-oauth-token" {
		t.Errorf("copied symlink target = %q, want %q", linkTarget, "antigravity-oauth-token")
	}

	// The whole point of staging the copy: the CLI must be able to create
	// new subdirectories (e.g. "brain", "conversations") that never
	// existed on the host, which a read-only bind mount of the original
	// host directory would reject with "permission denied".
	newStateDir := filepath.Join(dst, "antigravity-cli", "brain")
	if err := os.MkdirAll(newStateDir, 0755); err != nil {
		t.Fatalf("mkdir in copied tree should succeed: %v", err)
	}
}

func TestDockerRuntimeHomeBindMount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test in short mode")
	}
	ctx := context.Background()
	rt, err := NewDockerRuntime(DockerConfig{})
	if err != nil {
		t.Fatalf("Failed to create docker runtime: %v", err)
	}
	
	homeDir := t.TempDir()
	
	if err := rt.Prewarm(ctx); err != nil {
		t.Fatalf("Prewarm failed: %v", err)
	}
	
	req := CommandRequest{
		Command:     []string{"bash", "-c", "echo 'test-home-content' > /home/agent/test_home_file.txt"},
		SessionMounts: map[string]string{
			"/home/agent": homeDir,
		},
		Timeout:     15 * time.Second,
	}
	
	res, err := rt.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("Run failed with exit code %d. Stderr: %s", res.ExitCode, res.Stderr)
	}
	
	hostFilePath := filepath.Join(homeDir, "test_home_file.txt")
	data, err := os.ReadFile(hostFilePath)
	if err != nil {
		t.Fatalf("Failed to read file from host home dir mount: %v", err)
	}
	

	if content := string(data); strings.TrimSpace(content) != "test-home-content" {
		t.Errorf("Expected content 'test-home-content', got %q", content)
	}
}

func TestDockerRuntimeCredentialFilesSessionMountOverlap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test in short mode")
	}
	ctx := context.Background()
	rt, err := NewDockerRuntime(DockerConfig{})
	if err != nil {
		t.Fatalf("Failed to create docker runtime: %v", err)
	}

	sessionDir := t.TempDir()

	if err := rt.Prewarm(ctx); err != nil {
		t.Fatalf("Prewarm failed: %v", err)
	}

	req := CommandRequest{
		// The command reads the credential file then modifies it to test extraction
		Command: []string{"bash", "-c", "cat /home/agent/.claude/.credentials.json && echo -n 'updated-token' > /home/agent/.claude/.credentials.json"},
		SessionMounts: map[string]string{
			"/home/agent/.claude": sessionDir,
		},
		CredentialFiles: map[string]string{
			"/home/agent/.claude/.credentials.json": "injected-token-123",
		},
		Timeout: 15 * time.Second,
	}

	res, err := rt.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("Run failed with exit code %d. Stderr: %s", res.ExitCode, res.Stderr)
	}

	// Verify the container saw the initial credential
	if out := strings.TrimSpace(res.Stdout); out != "injected-token-123" {
		t.Errorf("Expected container stdout 'injected-token-123', got %q", out)
	}

	// Verify the updated credential file was written to the host session directory
	hostFilePath := filepath.Join(sessionDir, ".credentials.json")
	data, err := os.ReadFile(hostFilePath)
	if err != nil {
		t.Fatalf("Failed to read credential file from host session dir: %v", err)
	}

	if content := string(data); content != "updated-token" {
		t.Errorf("Expected host file content 'updated-token', got %q", content)
	}

	// Verify the UpdatedCredentialFiles was populated correctly
	if res.UpdatedCredentialFiles["/home/agent/.claude/.credentials.json"] != "updated-token" {
		t.Errorf("Expected UpdatedCredentialFiles to contain 'updated-token', got %v", res.UpdatedCredentialFiles)
	}
}
