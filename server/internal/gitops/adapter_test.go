package gitops

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockGitProvider struct {
	createdBranch string
	commitMessage string
	pushToken     string
	prTitle       string
	prHead        string
	prBase        string
}

func (m *mockGitProvider) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	return "main", nil
}

func (m *mockGitProvider) CreateBranch(ctx context.Context, localPath, branchName string) error {
	m.createdBranch = branchName
	return nil
}

func (m *mockGitProvider) CommitAndPush(ctx context.Context, localPath, message, token string) error {
	m.commitMessage = message
	m.pushToken = token
	return nil
}

func (m *mockGitProvider) CreatePR(ctx context.Context, owner, repo, title, head, base, body, token string) (string, error) {
	m.prTitle = title
	m.prHead = head
	m.prBase = base
	return "https://github.com/" + owner + "/" + repo + "/pull/1", nil
}

func (m *mockGitProvider) ListRepos(ctx context.Context, token string) ([]models.RemoteRepository, error) {
	return nil, nil
}

func (m *mockGitProvider) ValidateToken(ctx context.Context, token string) error {
	return nil
}

type mockRepoLookup struct {
	repo *models.Repository
}

func (m *mockRepoLookup) GetByURL(ctx context.Context, repoURL string) (*models.Repository, error) {
	if m.repo != nil && m.repo.URL == repoURL {
		return m.repo, nil
	}
	return nil, errors.New("not found")
}

func TestGitOpsAdapter(t *testing.T) {
	provider := &mockGitProvider{}
	repoDb := &mockRepoLookup{
		repo: &models.Repository{
			ID:        "repo-123",
			URL:       "https://github.com/test-owner/test-repo.git",
			Token:     "ghp_test_token",
			Branch:    "main",
			ProjectID: "project-123",
		},
	}
	tmpDir, err := os.MkdirTemp("", "adapter-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	adapter := NewGitOpsAdapter(provider, repoDb, tmpDir)

	ctx := context.Background()
	ctx = observability.WithTaskID(ctx, "task-456")

	// Test CreateBranch
	err = adapter.CreateBranch(ctx, "https://github.com/test-owner/test-repo.git", "feature-branch")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if provider.createdBranch != "feature-branch" {
		t.Errorf("expected feature-branch, got %s", provider.createdBranch)
	}

	// Test CommitAndPush
	files := map[string]string{
		"file1.txt": "hello world",
	}
	err = adapter.CommitAndPush(ctx, "https://github.com/test-owner/test-repo.git", "feature-branch", "commit msg", files)
	if err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}
	if provider.commitMessage != "commit msg" {
		t.Errorf("expected commit msg, got %s", provider.commitMessage)
	}
	if provider.pushToken != "ghp_test_token" {
		t.Errorf("expected ghp_test_token, got %s", provider.pushToken)
	}

	// Test CreatePullRequest
	prURL, err := adapter.CreatePullRequest(ctx, "https://github.com/test-owner/test-repo.git", "feature-branch", "title", "body")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if prURL != "https://github.com/test-owner/test-repo/pull/1" {
		t.Errorf("expected URL, got %s", prURL)
	}
	if provider.prTitle != "title" {
		t.Errorf("expected title, got %s", provider.prTitle)
	}
	if provider.prHead != "feature-branch" {
		t.Errorf("expected feature-branch, got %s", provider.prHead)
	}
	if provider.prBase != "main" {
		t.Errorf("expected main, got %s", provider.prBase)
	}
}
