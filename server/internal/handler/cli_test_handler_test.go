package handler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCredentialFile_NormalPathWritesInsideBaseDir(t *testing.T) {
	baseDir := t.TempDir()

	writeCredentialFile(baseDir, ".claude.json", `{"token":"abc"}`)

	content, err := os.ReadFile(filepath.Join(baseDir, ".claude.json"))
	if err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}
	if string(content) != `{"token":"abc"}` {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestWriteCredentialFile_NestedPathWritesInsideBaseDir(t *testing.T) {
	baseDir := t.TempDir()

	writeCredentialFile(baseDir, ".config/codex/auth.json", `{"token":"abc"}`)

	if _, err := os.Stat(filepath.Join(baseDir, ".config", "codex", "auth.json")); err != nil {
		t.Fatalf("expected nested file to be written: %v", err)
	}
}

func TestWriteCredentialFile_DotDotTraversalRejected(t *testing.T) {
	baseDir := t.TempDir()

	writeCredentialFile(baseDir, "../escaped.json", "malicious")

	if _, err := os.Stat(filepath.Join(filepath.Dir(baseDir), "escaped.json")); err == nil {
		t.Fatal("expected traversal write to be rejected, but file was created outside baseDir")
	}
}

func TestWriteCredentialFile_SiblingPrefixCollisionRejected(t *testing.T) {
	// Regression test: baseDir="/tmp/x/1" vs a resolved path under the
	// sibling "/tmp/x/12" used to pass a naive
	// strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(baseDir))
	// check purely because the strings share a prefix, even though
	// "/tmp/x/12" is not inside "/tmp/x/1" at all.
	parent := t.TempDir()
	baseDir := filepath.Join(parent, "1")
	sibling := filepath.Join(parent, "12")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	// relPath crafted so filepath.Join(baseDir, relPath) resolves into the
	// sibling directory instead of staying inside baseDir.
	relPath := "../12/escaped.json"
	writeCredentialFile(baseDir, relPath, "malicious")

	if _, err := os.Stat(filepath.Join(sibling, "escaped.json")); err == nil {
		t.Fatal("expected sibling-prefix-collision write to be rejected, but file was created in the sibling dir")
	}
}
