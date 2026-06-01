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

type GitOpsAdapter struct {
	provider GitProvider
	repoDb   RepositoryLookup
	rootPath string
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

func (a *GitOpsAdapter) localPath(ctx context.Context, repoID string) string {
	if taskID := observability.TaskID(ctx); taskID != "" {
		return filepath.Join(a.rootPath, taskID)
	}
	return filepath.Join(a.rootPath, repoID)
}

func (a *GitOpsAdapter) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	return a.provider.CloneRepo(ctx, repoURL, token, branch, localPath)
}

func (a *GitOpsAdapter) CreateBranch(ctx context.Context, repoURL, branchName string) error {
	repo, err := a.repoDb.GetByURL(ctx, repoURL)
	if err != nil {
		return err
	}
	path := a.localPath(ctx, repo.ID)
	return a.provider.CreateBranch(ctx, path, branchName)
}

func (a *GitOpsAdapter) CommitAndPush(ctx context.Context, repoURL, branchName, message string, files map[string]string) error {
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

	return a.provider.CommitAndPush(ctx, path, message, repo.Token)
}

func (a *GitOpsAdapter) CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error) {
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
	return a.provider.CreatePR(ctx, owner, repoName, title, branchName, repo.Branch, body, repo.Token)
}
