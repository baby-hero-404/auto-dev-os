package sandbox

import (
	"os"
	"path/filepath"
	"testing"
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
