package gitops

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
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
	repoURL, repo, err := a.lookupRepository(ctx, repoURL)
	if err != nil {
		return "", fmt.Errorf("lookup repo %s: %w", repoURL, err)
	}
	provider, token := a.providerAndTokenForRepo(ctx, repo)
	return provider.CloneRepo(ctx, repoURL, token, branch, localPath)
}

func (a *GitOpsAdapter) CreateBranch(ctx context.Context, repoURL, branchName string) error {
	_, repo, err := a.lookupRepository(ctx, repoURL)
	if err != nil {
		return err
	}
	path := a.localPath(ctx, repo.ID)
	return a.provider.CreateBranch(ctx, path, branchName)
}

func (a *GitOpsAdapter) CommitAndPush(ctx context.Context, repoURL, branchName, message string, files map[string]string, agentRole string) error {
	_, repo, err := a.lookupRepository(ctx, repoURL)
	if err != nil {
		return err
	}
	path := a.localPath(ctx, repo.ID)

	// If files are explicitly provided (which is optional but supported), write them to local path.
	for file, content := range files {
		fullPath, err := safeRepoPath(path, file)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("create file directory %s: %w", file, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", file, err)
		}
	}

	provider, token := a.providerAndTokenForRepo(ctx, repo)
	return provider.CommitAndPush(ctx, path, message, token, agentRole)
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
	repoURL, repo, err := a.lookupRepository(ctx, repoURL)
	if err != nil {
		return "", err
	}

	path := a.localPath(ctx, repo.ID)
	baseBranch := repo.Branch
	if baseBranch == "" {
		baseBranch = "main" // fallback
	}

	// Local check to see if there are any commits between base branch and task branch.
	cmd := exec.CommandContext(ctx, "git", "-C", path, "log", fmt.Sprintf("%s..%s", baseBranch, branchName), "--oneline")
	if output, err := cmd.CombinedOutput(); err == nil {
		if len(strings.TrimSpace(string(output))) == 0 {
			observability.Info(ctx, "skipping PR creation because branch has no commits relative to base branch",
				"branch", branchName, "base", baseBranch)
			return fmt.Sprintf("no changes detected (branch %s has no commits relative to %s)", branchName, baseBranch), nil
		}
	} else {
		// If baseBranch isn't main but master, try master.
		if baseBranch == "main" {
			cmd = exec.CommandContext(ctx, "git", "-C", path, "log", fmt.Sprintf("master..%s", branchName), "--oneline")
			if output, err2 := cmd.CombinedOutput(); err2 == nil && len(strings.TrimSpace(string(output))) == 0 {
				observability.Info(ctx, "skipping PR creation because branch has no commits relative to master",
					"branch", branchName)
				return fmt.Sprintf("no changes detected (branch %s has no commits relative to master)", branchName), nil
			}
		}
	}

	owner, repoName, err := parseRepoOwnerName(repoURL)
	if err != nil {
		return "", err
	}

	// Call underlying provider to create PR
	// head is branchName, base is the default branch from the repository model
	provider, token := a.providerAndTokenForRepo(ctx, repo)
	prURL, err := provider.CreatePR(ctx, owner, repoName, title, branchName, baseBranch, body, token)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no commits between") {
			observability.Info(ctx, "github API reported no commits between branches; returning friendly status",
				"branch", branchName, "base", baseBranch)
			return fmt.Sprintf("no changes detected (branch %s has no commits relative to %s)", branchName, baseBranch), nil
		}
		return "", err
	}
	return prURL, nil
}

func (a *GitOpsAdapter) lookupRepository(ctx context.Context, repoURL string) (string, *models.Repository, error) {
	if a.repoDb == nil {
		return repoURL, nil, fmt.Errorf("repository lookup is not configured")
	}
	original := strings.TrimSpace(repoURL)
	normalized := original
	if candidate, err := NormalizeGitURLToHTTPS(original); err == nil {
		normalized = candidate
	}
	repo, err := a.repoDb.GetByURL(ctx, normalized)
	if err == nil {
		return normalized, repo, nil
	}
	if original != "" && original != normalized {
		if repo, originalErr := a.repoDb.GetByURL(ctx, original); originalErr == nil {
			return normalized, repo, nil
		}
	}
	return normalized, nil, err
}

func safeRepoPath(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("file path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("file path %q must be relative", rel)
	}
	cleanRel := filepath.Clean(rel)
	parts := strings.Split(cleanRel, string(os.PathSeparator))
	if len(parts) > 0 && parts[0] == ".git" {
		return "", fmt.Errorf("file path %q is not allowed to modify .git internals", rel)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(absRoot, cleanRel)
	if target != absRoot && !strings.HasPrefix(target, absRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("file path %q escapes repository workspace", rel)
	}

	current := absRoot
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil && (info.Mode()&os.ModeSymlink) != 0 {
			return "", fmt.Errorf("file path %q contains a symlink component %q", rel, part)
		}
	}

	return target, nil
}

func parseRepoOwnerName(repoURL string) (string, string, error) {
	parseURL := repoURL
	if normalized, err := NormalizeGitURLToHTTPS(repoURL); err == nil {
		parseURL = normalized
	}
	parsed, err := url.Parse(parseURL)
	if err != nil {
		return "", "", fmt.Errorf("parse repo URL %s: %w", repoURL, err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid repository URL path: %s", parsed.Path)
	}
	owner := parts[0]
	repoName := strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repoName == "" {
		return "", "", fmt.Errorf("invalid repository URL path: %s", parsed.Path)
	}
	return owner, repoName, nil
}
