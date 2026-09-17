package prompts

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type MockContextEngine struct {
	called bool
}

func (m *MockContextEngine) RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error) {
	m.called = true
	return []models.ContextSnippet{{
		Path:      "server/internal/service/task.go",
		StartLine: 10,
		EndLine:   20,
		Content:   "func AnalyzeTask() {}",
		Relevance: 9.5,
		Retriever: "ast_context_engine",
	}}, nil
}

func (m *MockContextEngine) GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error) {
	return "main.go:\n  def Main", nil
}

func (m *MockContextEngine) IndexWorkspace(ctx context.Context) error {
	return nil
}

func (m *MockContextEngine) Close() error {
	return nil
}

func (m *MockContextEngine) GetGlobalCacheDir() string {
	return ""
}

func (m *MockContextEngine) BuildGlobalCache(repoAbsPath string, repoName string, commitHash string) error {
	return nil
}

func (m *MockContextEngine) InitLocalCache(wsRoot string, repoCommits []provider.RepoCommitInfo) error {
	return nil
}
