package gitops

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GitProvider interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	CreateBranch(ctx context.Context, localPath, branchName string) error
	CommitAndPush(ctx context.Context, localPath, message, token, agentRole string) error
	CreatePR(ctx context.Context, owner, repo, title, head, base, body, token string) (string, error)
	ListRepos(ctx context.Context, token string) ([]models.RemoteRepository, error)
	ValidateToken(ctx context.Context, token string) error
}

// NormalizeGitURLToHTTPS converts a Git SSH URL (scp-like or standard ssh://) to a standard HTTPS URL.
// If the input is already an HTTP/HTTPS URL, it returns it unchanged.
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

	return rawURL, nil
}
