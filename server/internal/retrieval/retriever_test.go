package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleFileRetrieverRetrieveContextBySemanticTerms(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "server/internal/service/user.go", `package service

func ValidateUserProfileAvatar(input string) error {
	return nil
}
`)
	writeTestFile(t, root, "web/src/app/page.tsx", `export default function Home() {
	return <main>Dashboard</main>;
}
`)

	retriever := NewSimpleFileRetriever(root)
	snippets, err := retriever.RetrieveContext(context.Background(), "Fix avatar validation in user profile service", 8)
	if err != nil {
		t.Fatalf("RetrieveContext returned error: %v", err)
	}
	if len(snippets) == 0 {
		t.Fatal("expected snippets")
	}
	if snippets[0].Path != "server/internal/service/user.go" {
		t.Fatalf("expected user service first, got %q", snippets[0].Path)
	}
	if snippets[0].Retriever != "semantic_file" {
		t.Fatalf("unexpected retriever %q", snippets[0].Retriever)
	}
}

func TestSimpleFileRetrieverPrioritizesExplicitPath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "server/main.go", "package main\nfunc main() {}\n")
	writeTestFile(t, root, "server/internal/service/main_service.go", "package service\nfunc StartMainService() {}\n")

	retriever := NewSimpleFileRetriever(root)
	snippets, err := retriever.RetrieveContext(context.Background(), "inspect server/main.go and startup behavior", 8)
	if err != nil {
		t.Fatalf("RetrieveContext returned error: %v", err)
	}
	if len(snippets) == 0 {
		t.Fatal("expected snippets")
	}
	if snippets[0].Path != "server/main.go" {
		t.Fatalf("expected explicit path first, got %q", snippets[0].Path)
	}
}

func TestSimpleFileRetrieverIgnoresTraversalPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "server/main.go", "package main\nfunc main() {}\n")

	retriever := NewSimpleFileRetriever(root)
	snippets, err := retriever.RetrieveContext(context.Background(), "inspect ../secret.txt", 8)
	if err != nil {
		t.Fatalf("RetrieveContext returned error: %v", err)
	}
	if len(snippets) != 0 {
		t.Fatalf("expected no snippets for traversal-only query, got %d", len(snippets))
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		expected bool
	}{
		{"logs", "logs", true},
		{"logs", "logs/call-1", true},
		{"artifacts", "artifacts", true},
		{"artifacts", "artifacts/findings", true},
		{"openspec", "openspec", true},
		{"openspec", "openspec/spec.md", true},
		{"openspec", "code/repos/test/main/openspec", false},
		{"logs", "code/repos/test/main/logs", false},
		{"artifacts", "code/repos/test/main/artifacts", false},
		{"node_modules", "node_modules", true},
		{"node_modules", "code/repos/test/main/node_modules", true},
	}

	for _, tc := range tests {
		got := shouldSkipDir(tc.name, tc.rel)
		if got != tc.expected {
			t.Errorf("shouldSkipDir(%q, %q) = %v; expected %v", tc.name, tc.rel, got, tc.expected)
		}
	}
}
