package gitops

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type RepositoryLookup interface {
	GetByURL(ctx context.Context, repoURL string) (*models.Repository, error)
}

type GitAccountLookup interface {
	GetByID(ctx context.Context, id string) (*models.GitAccount, error)
}

type GitOpsAdapter struct {
	provider     GitProvider
	repoDb       RepositoryLookup
	gitAccountDb GitAccountLookup
	rootPath     string
}

func NewGitOpsAdapter(provider GitProvider, repoDb RepositoryLookup, rootPath string) *GitOpsAdapter {
	if rootPath == "" {
		rootPath = "/tmp/auto-code-os/workspaces"
	}
	return &GitOpsAdapter{
		provider: provider,
		repoDb:   repoDb,
		rootPath: rootPath,
	}
}

func (a *GitOpsAdapter) SetGitAccountLookup(lookup GitAccountLookup) {
	a.gitAccountDb = lookup
}

func (a *GitOpsAdapter) localPath(ctx context.Context, repoID string) string {
	if taskID := observability.TaskID(ctx); taskID != "" {
		return filepath.Join(a.rootPath, taskID)
	}
	return filepath.Join(a.rootPath, repoID)
}

func (a *GitOpsAdapter) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	return a.provider.CloneRepo(ctx, repoURL, token, branch, localPath)
}

// CloneForTask looks up the repository by URL, resolves credentials from the
// linked GitAccount (falling back to repo.Token), and clones using the correct
// provider and token. This keeps credential resolution out of the orchestrator.
func (a *GitOpsAdapter) CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error) {
	normalized, err := NormalizeGitURLToHTTPS(repoURL)
	if err == nil {
		repoURL = normalized
	}
	repo, err := a.repoDb.GetByURL(ctx, repoURL)
	if err != nil {
		return "", fmt.Errorf("lookup repo %s: %w", repoURL, err)
	}
	provider, token := a.providerAndTokenForRepo(ctx, repo)
	return provider.CloneRepo(ctx, repoURL, token, branch, localPath)
}

func (a *GitOpsAdapter) CreateBranch(ctx context.Context, repoURL, branchName string) error {
	normalized, err := NormalizeGitURLToHTTPS(repoURL)
	if err == nil {
		repoURL = normalized
	}
	repo, err := a.repoDb.GetByURL(ctx, repoURL)
	if err != nil {
		return err
	}
	path := a.localPath(ctx, repo.ID)
	return a.provider.CreateBranch(ctx, path, branchName)
}

func (a *GitOpsAdapter) CommitAndPush(ctx context.Context, repoURL, branchName, message string, files map[string]string, agentRole string) error {
	normalized, err := NormalizeGitURLToHTTPS(repoURL)
	if err == nil {
		repoURL = normalized
	}
	repo, err := a.repoDb.GetByURL(ctx, repoURL)
	if err != nil {
		return err
	}
	path := a.localPath(ctx, repo.ID)

	// If files are explicitly provided (which is optional but supported), write them to local path.
	for file, content := range files {
		fullPath := filepath.Join(path, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("create file directory %s: %w", file, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", file, err)
		}
	}

	_, token := a.providerAndTokenForRepo(ctx, repo)
	return a.provider.CommitAndPush(ctx, path, message, token, agentRole)
}

func (a *GitOpsAdapter) providerAndTokenForRepo(ctx context.Context, repo *models.Repository) (GitProvider, string) {
	token := repo.Token
	if repo.GitAccountID == nil || *repo.GitAccountID == "" || a.gitAccountDb == nil {
		return a.provider, token
	}
	acc, err := a.gitAccountDb.GetByID(ctx, *repo.GitAccountID)
	if err != nil {
		return a.provider, token
	}
	if token == "" {
		token = acc.Token
	}
	if acc.BaseURL == "" {
		return a.provider, token
	}
	return NewGitHubProvider(acc.BaseURL), token
}

func (a *GitOpsAdapter) CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error) {
	normalized, err := NormalizeGitURLToHTTPS(repoURL)
	if err == nil {
		repoURL = normalized
	}
	repo, err := a.repoDb.GetByURL(ctx, repoURL)
	if err != nil {
		return "", err
	}

	// Parse owner and repo from repoURL
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parse repo URL %s: %w", repoURL, err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid repository URL path: %s", parsed.Path)
	}
	owner := parts[0]
	repoName := strings.TrimSuffix(parts[1], ".git")

	// Call underlying provider to create PR
	// head is branchName, base is the default branch from the repository model
	provider, token := a.providerAndTokenForRepo(ctx, repo)
	return provider.CreatePR(ctx, owner, repoName, title, branchName, repo.Branch, body, token)
}
