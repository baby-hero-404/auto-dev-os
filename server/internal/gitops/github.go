package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GitHubProvider struct {
	client *http.Client
}

func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

func (p *GitHubProvider) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	cloneURL := repoURL
	if token != "" {
		withToken, err := githubURLWithToken(repoURL, token)
		if err != nil {
			return "", err
		}
		cloneURL = withToken
	}

	var cloneCmd *exec.Cmd
	if branch != "" {
		cloneCmd = exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", cloneURL, localPath)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			outStr := string(output)
			if strings.Contains(outStr, "Remote branch") || strings.Contains(outStr, "Could not find remote branch") {
				// Retry with the default branch
				os.RemoveAll(localPath)
				fallbackCmd := exec.CommandContext(ctx, "git", "clone", "--single-branch", cloneURL, localPath)
				if fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput(); fallbackErr != nil {
					return "", fmt.Errorf("git clone: %w: %s (fallback failed: %s)", err, sanitizeToken(outStr, token), sanitizeToken(string(fallbackOutput), token))
				}
			} else {
				return "", fmt.Errorf("git clone: %w: %s", err, sanitizeToken(outStr, token))
			}
		}
	} else {
		cloneCmd = exec.CommandContext(ctx, "git", "clone", "--single-branch", cloneURL, localPath)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone: %w: %s", err, sanitizeToken(string(output), token))
		}
	}

	actualBranchCmd := exec.CommandContext(ctx, "git", "-C", localPath, "rev-parse", "--abbrev-ref", "HEAD")
	actualBranchOutput, err := actualBranchCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get active branch name: %w: %s", err, string(actualBranchOutput))
	}
	actualBranch := strings.TrimSpace(string(actualBranchOutput))
	return actualBranch, nil
}

func (p *GitHubProvider) CreateBranch(ctx context.Context, localPath, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", localPath, "checkout", "-b", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout branch: %w: %s", err, string(output))
	}
	return nil
}

func (p *GitHubProvider) CommitAndPush(ctx context.Context, localPath, message, token string) error {
	commands := [][]string{
		{"git", "-C", localPath, "add", "."},
		{"git", "-C", localPath, "commit", "-m", message},
		{"git", "-C", localPath, "push", "origin", "HEAD"},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, sanitizeToken(string(output), token))
		}
	}
	return nil
}

func (p *GitHubProvider) CreatePR(ctx context.Context, owner, repo, title, head, base, body, token string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"title": title,
		"head":  head,
		"base":  base,
		"body":  body,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	p.authorize(req, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github create pr returned %s", resp.Status)
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.HTMLURL, nil
}

func (p *GitHubProvider) ListRepos(ctx context.Context, token string) ([]models.RemoteRepository, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
	if err != nil {
		return nil, err
	}
	p.authorize(req, token)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github list repos returned %s", resp.Status)
	}
	var raw []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	repos := make([]models.RemoteRepository, 0, len(raw))
	for _, r := range raw {
		repos = append(repos, models.RemoteRepository{
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return repos, nil
}

func (p *GitHubProvider) ValidateToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return err
	}
	p.authorize(req, token)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github token validation returned %s", resp.Status)
	}
	return nil
}

func (p *GitHubProvider) authorize(req *http.Request, token string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func githubURLWithToken(rawURL, token string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("authenticated clone requires https repository URL")
	}
	parsed.User = url.UserPassword("x-access-token", token)
	return parsed.String(), nil
}

func sanitizeToken(value, token string) string {
	if token == "" {
		return value
	}
	return strings.ReplaceAll(value, token, "[redacted]")
}
