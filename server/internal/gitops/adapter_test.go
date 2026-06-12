package gitops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockGitProvider struct {
	createdBranch string
	commitMessage string
	commitPath    string
	pushToken     string
	prTitle       string
	prHead        string
	prBase        string
	clonedURL     string
}

func (m *mockGitProvider) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	m.clonedURL = repoURL
	return "main", nil
}

func (m *mockGitProvider) CreateBranch(ctx context.Context, localPath, branchName string) error {
	m.createdBranch = branchName
	return nil
}

func (m *mockGitProvider) CommitAndPush(ctx context.Context, localPath, message, token, agentRole string) error {
	m.commitPath = localPath
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

type mockGitAccountLookup struct {
	account *models.GitAccount
}

func (m *mockGitAccountLookup) GetByID(ctx context.Context, id string) (*models.GitAccount, error) {
	if m.account != nil && m.account.ID == id {
		return m.account, nil
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
	err = adapter.CommitAndPush(ctx, "https://github.com/test-owner/test-repo.git", "feature-branch", "commit msg", files, "backend")
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

func TestGitOpsAdapter_ProviderAndTokenForRepo_UsesLinkedAccountCredentials(t *testing.T) {
	accountID := "git-account-1"
	adapter := NewGitOpsAdapter(&mockGitProvider{}, &mockRepoLookup{}, "")
	adapter.SetGitAccountLookup(&mockGitAccountLookup{
		account: &models.GitAccount{
			ID:      accountID,
			BaseURL: "https://github.example.com/api/v3",
			Token:   "account-token",
		},
	})

	provider, token := adapter.providerAndTokenForRepo(context.Background(), &models.Repository{
		GitAccountID: &accountID,
	})

	ghProvider, ok := provider.(*GitHubProvider)
	if !ok {
		t.Fatalf("expected GitHubProvider, got %T", provider)
	}
	if ghProvider.baseURL != "https://github.example.com/api/v3" {
		t.Errorf("expected linked account baseURL, got %q", ghProvider.baseURL)
	}
	if token != "account-token" {
		t.Errorf("expected linked account token, got %q", token)
	}
}

func TestGitOpsAdapter_CommitAndPush_UsesLinkedAccountToken(t *testing.T) {
	accountID := "git-account-1"
	provider := &mockGitProvider{}
	repoDb := &mockRepoLookup{
		repo: &models.Repository{
			ID:           "repo-123",
			URL:          "https://github.com/test-owner/test-repo.git",
			Token:        "",
			Branch:       "main",
			GitAccountID: &accountID,
		},
	}
	tmpDir := t.TempDir()
	adapter := NewGitOpsAdapter(provider, repoDb, tmpDir)
	adapter.SetGitAccountLookup(&mockGitAccountLookup{
		account: &models.GitAccount{
			ID:    accountID,
			Token: "account-token",
		},
	})
	ctx := observability.WithTaskID(context.Background(), "task-456")

	if err := adapter.CommitAndPush(ctx, "https://github.com/test-owner/test-repo.git", "feature", "commit msg", nil, "backend"); err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}
	if provider.pushToken != "account-token" {
		t.Fatalf("expected linked account token, got %q", provider.pushToken)
	}
	if provider.commitPath != filepath.Join(tmpDir, "task-456") {
		t.Fatalf("expected task workspace path, got %q", provider.commitPath)
	}
}

func TestGitOpsAdapter_CommitAndPush_RejectsEscapingFilePath(t *testing.T) {
	provider := &mockGitProvider{}
	repoDb := &mockRepoLookup{
		repo: &models.Repository{
			ID:  "repo-123",
			URL: "https://github.com/test-owner/test-repo.git",
		},
	}
	adapter := NewGitOpsAdapter(provider, repoDb, t.TempDir())

	err := adapter.CommitAndPush(context.Background(), "https://github.com/test-owner/test-repo.git", "feature", "commit msg", map[string]string{
		"../outside.txt": "escape",
	}, "backend")
	if err == nil {
		t.Fatal("expected path escape error")
	}
	if provider.commitMessage != "" {
		t.Fatal("commit should not run after path validation failure")
	}
}

func TestGitOpsAdapter_LookupRepository_FallsBackToOriginalURL(t *testing.T) {
	provider := &mockGitProvider{}
	sshURL := "git@github.com:test-owner/test-repo.git"
	repoDb := &mockRepoLookup{
		repo: &models.Repository{
			ID:  "repo-123",
			URL: sshURL,
		},
	}
	adapter := NewGitOpsAdapter(provider, repoDb, t.TempDir())

	if err := adapter.CreateBranch(context.Background(), sshURL, "feature"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if provider.createdBranch != "feature" {
		t.Fatalf("expected branch to be created after original URL fallback")
	}
}

func TestGitOpsAdapter_CreatePullRequest_UsesMainWhenRepoBranchEmpty(t *testing.T) {
	provider := &mockGitProvider{}
	repoDb := &mockRepoLookup{
		repo: &models.Repository{
			ID:  "repo-123",
			URL: "https://github.com/test-owner/test-repo.git",
		},
	}
	adapter := NewGitOpsAdapter(provider, repoDb, t.TempDir())

	_, err := adapter.CreatePullRequest(context.Background(), "https://github.com/test-owner/test-repo.git", "feature", "title", "body")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if provider.prBase != "main" {
		t.Fatalf("expected fallback base branch main, got %q", provider.prBase)
	}
}

func TestGitOpsAdapter_CreatePullRequest_ParsesOriginalSSHURL(t *testing.T) {
	provider := &mockGitProvider{}
	sshURL := "git@github.com:test-owner/test-repo.git"
	repoDb := &mockRepoLookup{
		repo: &models.Repository{
			ID:     "repo-123",
			URL:    sshURL,
			Branch: "main",
		},
	}
	adapter := NewGitOpsAdapter(provider, repoDb, t.TempDir())

	prURL, err := adapter.CreatePullRequest(context.Background(), sshURL, "feature", "title", "body")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if prURL != "https://github.com/test-owner/test-repo/pull/1" {
		t.Fatalf("expected PR URL from parsed SSH repo, got %q", prURL)
	}
}
