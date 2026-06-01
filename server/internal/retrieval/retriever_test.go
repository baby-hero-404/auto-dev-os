package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleFileRetrieverRetrieveContext(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	retriever := NewSimpleFileRetriever(root)
	snippets, err := retriever.RetrieveContext(context.Background(), "inspect server/main.go and missing.go", 5)
	if err != nil {
		t.Fatalf("RetrieveContext returned error: %v", err)
	}
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(snippets))
	}
	if snippets[0].Path != "server/main.go" {
		t.Fatalf("unexpected path %q", snippets[0].Path)
	}
}

func TestSimpleFileRetrieverRejectsTraversal(t *testing.T) {
	retriever := NewSimpleFileRetriever(t.TempDir())
	if _, err := retriever.readSnippet("../secret.txt"); err == nil {
		t.Fatal("expected traversal error")
	}
}
