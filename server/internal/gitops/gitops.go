package gitops

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GitProvider interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	CreateBranch(ctx context.Context, localPath, branchName string) error
	CommitAndPush(ctx context.Context, localPath, message, token, agentRole string) error
	CreatePR(ctx context.Context, owner, repo, title, head, base, body, token string) (string, error)
	MergePR(ctx context.Context, owner, repo, prURL, token string) error
	IsPRMerged(ctx context.Context, owner, repo, prURL, token string) (bool, error)
	ListRepos(ctx context.Context, token string) ([]models.RemoteRepository, error)
	ValidateToken(ctx context.Context, token string) error
}

// NormalizeGitURLToHTTPS converts a Git SSH URL (scp-like or standard ssh://) to a standard HTTPS URL.
// If the input is already an HTTP/HTTPS URL, it returns it unchanged.
// It also autocompletes domain-only paths (e.g. github.com/owner/repo) to HTTPS and validates the URL format.
func NormalizeGitURLToHTTPS(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	// Case 1: SCP-like SSH format, e.g. git@github.com:owner/repo.git
	if strings.Contains(rawURL, "@") && !strings.Contains(rawURL, "://") {
		parts := strings.SplitN(rawURL, "@", 2)
		hostAndPath := parts[1]
		colonIdx := strings.Index(hostAndPath, ":")
		if colonIdx != -1 {
			host := hostAndPath[:colonIdx]
			path := hostAndPath[colonIdx+1:]
			return fmt.Sprintf("https://%s/%s", host, path), nil
		}
	}

	// Case 2: Standard SSH scheme, e.g. ssh://git@github.com/owner/repo.git or git+ssh://...
	if strings.HasPrefix(rawURL, "ssh://") || strings.HasPrefix(rawURL, "git+ssh://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", err
		}
		host := u.Host
		if strings.Contains(host, "@") {
			parts := strings.SplitN(host, "@", 2)
			host = parts[1]
		}
		path := strings.TrimPrefix(u.Path, "/")
		return fmt.Sprintf("https://%s/%s", host, path), nil
	}

	// Case 3: Already HTTP/HTTPS
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL, nil
	}

	// Case 4: Existing directory on disk (for local testing)
	if fi, err := os.Stat(rawURL); err == nil && fi.IsDir() {
		return rawURL, nil
	}

	// Case 5: Domain name path (e.g., github.com/owner/repo)
	firstSlash := strings.Index(rawURL, "/")
	if firstSlash != -1 {
		hostPart := rawURL[:firstSlash]
		if strings.Contains(hostPart, ".") {
			return "https://" + rawURL, nil
		}
	}

	return "", fmt.Errorf("invalid repository URL: must be a valid HTTP/HTTPS/SSH URL or an existing local path")
}

// gitCommand creates an exec.Cmd for git and disables credential prompts
// by injecting environment variables. This prevents the OS from popping up
// credential windows (like VS Code GitHub extension) during background operations.
func gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=false",
	)
	return cmd
}
