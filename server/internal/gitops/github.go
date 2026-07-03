package gitops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

const defaultGitHubAPIURL = "https://api.github.com"

type GitHubProvider struct {
	client  *http.Client
	baseURL string
}

// NewGitHubProvider creates a GitHub API client. Pass an empty baseURL to use
// the public GitHub API (https://api.github.com). For GitHub Enterprise, pass
// the enterprise API base URL (e.g. "https://github.example.com/api/v3").
func NewGitHubProvider(baseURL string) *GitHubProvider {
	if baseURL == "" {
		baseURL = defaultGitHubAPIURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &GitHubProvider{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: baseURL,
	}
}

func (p *GitHubProvider) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	var authArgs []string
	if token != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		authArgs = append(authArgs, "-c", "http.extraHeader=AUTHORIZATION: basic "+b64)
	}

	var cloneCmd *exec.Cmd
	if branch != "" {
		args := append(authArgs, "clone", "--branch", branch, "--single-branch", repoURL, localPath)
		cloneCmd = gitCommand(ctx, args...)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			outStr := string(output)
			if strings.Contains(outStr, "Remote branch") || strings.Contains(outStr, "Could not find remote branch") {
				// Retry with the default branch
				os.RemoveAll(localPath)
				fallbackArgs := append(authArgs, "clone", "--single-branch", repoURL, localPath)
				fallbackCmd := gitCommand(ctx, fallbackArgs...)
				if fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput(); fallbackErr != nil {
					return "", fmt.Errorf("git clone: %w: %s (fallback failed: %s)", err, sanitizeToken(outStr, token), sanitizeToken(string(fallbackOutput), token))
				}
			} else {
				return "", fmt.Errorf("git clone: %w: %s", err, sanitizeToken(outStr, token))
			}
		}
	} else {
		args := append(authArgs, "clone", "--single-branch", repoURL, localPath)
		cloneCmd = gitCommand(ctx, args...)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone: %w: %s", err, sanitizeToken(string(output), token))
		}
	}

	actualBranchCmd := gitCommand(ctx, "-C", localPath, "rev-parse", "--abbrev-ref", "HEAD")
	actualBranchOutput, err := actualBranchCmd.CombinedOutput()
	if err != nil {
		// Fallback for newly created empty repository where HEAD does not point to any commit yet
		targetBranch := branch
		if targetBranch == "" {
			fallbackCmd := gitCommand(ctx, "-C", localPath, "symbolic-ref", "--short", "HEAD")
			fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput()
			if fallbackErr == nil {
				targetBranch = strings.TrimSpace(string(fallbackOutput))
			}
		}
		if targetBranch == "" {
			targetBranch = "main" // absolute fallback
		}

		// Initialize empty repository so that it is no longer unborn and worktrees can be created.
		// Use user config to avoid "Please tell me who you are" git commit errors.
		_ = gitCommand(ctx, "-C", localPath, "config", "user.name", "AutoCodeOS Initializer").Run()
		_ = gitCommand(ctx, "-C", localPath, "config", "user.email", "init@autocodeos.local").Run()

		// Rename/checkout to the target branch.
		checkoutCmd := gitCommand(ctx, "-C", localPath, "checkout", "-B", targetBranch)
		_ = checkoutCmd.Run()

		// Create an initial commit with an empty README.md.
		_ = os.WriteFile(filepath.Join(localPath, "README.md"), []byte("# "+targetBranch+"\n"), 0644)
		_ = gitCommand(ctx, "-C", localPath, "add", "README.md").Run()

		commitCmd := gitCommand(ctx, "-C", localPath, "commit", "-m", "Initial commit")
		if commitOut, commitErr := commitCmd.CombinedOutput(); commitErr != nil {
			return "", fmt.Errorf("failed to create initial commit for empty repo: %w: %s", commitErr, string(commitOut))
		}

		actualBranchOutput = []byte(targetBranch)
	}
	actualBranch := strings.TrimSpace(string(actualBranchOutput))
	return actualBranch, nil
}

func (p *GitHubProvider) CreateBranch(ctx context.Context, localPath, branchName string) error {
	// Detach HEAD so we are not on the branch we want to delete.
	detachCmd := gitCommand(ctx, "-C", localPath, "checkout", "--detach")
	_ = detachCmd.Run()

	// Force delete the branch if it already exists.
	deleteCmd := gitCommand(ctx, "-C", localPath, "branch", "-D", branchName)
	_ = deleteCmd.Run()

	// Create and checkout the clean branch.
	cmd := gitCommand(ctx, "-C", localPath, "checkout", "-b", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout branch: %w: %s", err, string(output))
	}
	return nil
}

func (p *GitHubProvider) CommitAndPush(ctx context.Context, localPath, message, token, agentRole string) error {
	// Build git identity from the agent role performing this task.
	if agentRole == "" {
		agentRole = "agent"
	}
	gitName := fmt.Sprintf("AutoCodeOS [%s]", agentRole)
	gitEmail := fmt.Sprintf("%s@autocodeos.local", agentRole)
	identityCommands := [][]string{
		{"git", "-C", localPath, "config", "user.name", gitName},
		{"git", "-C", localPath, "config", "user.email", gitEmail},
	}
	for _, args := range identityCommands {
		cmd := gitCommand(ctx, args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, string(output))
		}
	}

	// Stage all changes.
	addCmd := gitCommand(ctx, "-C", localPath, "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w: %s", err, sanitizeToken(string(output), token))
	}

	// Check if there is anything to commit (avoid crash on clean working tree).
	statusCmd := gitCommand(ctx, "-C", localPath, "diff", "--cached", "--quiet")
	hasChanges := true
	if err := statusCmd.Run(); err == nil {
		hasChanges = false
	}

	// Commit if there are changes.
	if hasChanges {
		commitCmd := gitCommand(ctx, "-C", localPath, "commit", "-m", message)
		if output, err := commitCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit: %w: %s", err, sanitizeToken(string(output), token))
		}
	}

	// If the remote is empty, we must push the default branch first so that the remote base branch exists
	// and a PR can be successfully created.
	var authArgs []string
	if token != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		authArgs = append(authArgs, "-c", "http.extraHeader=AUTHORIZATION: basic "+b64)
	}

	lsArgs := append([]string{"-C", localPath}, authArgs...)
	lsArgs = append(lsArgs, "ls-remote", "--heads", "origin")
	lsCmd := gitCommand(ctx, lsArgs...)
	lsOut, err := lsCmd.CombinedOutput()
	if err == nil && len(strings.TrimSpace(string(lsOut))) == 0 {
		// Remote is empty. Find and push local default branch (main or master) to origin first.
		for _, b := range []string{"main", "master"} {
			checkCmd := gitCommand(ctx, "-C", localPath, "show-ref", "--quiet", "refs/heads/"+b)
			if err := checkCmd.Run(); err == nil {
				pushArgs := append([]string{"-C", localPath}, authArgs...)
				pushArgs = append(pushArgs, "push", "-u", "origin", b)
				_ = gitCommand(ctx, pushArgs...).Run()
				break
			}
		}
	}

	// Always force-push origin HEAD to ensure the remote branch is created or updated.
	var pushArgs []string
	pushArgs = append(pushArgs, "-C", localPath)
	if token != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		pushArgs = append(pushArgs, "-c", "http.extraHeader=AUTHORIZATION: basic "+b64)
	}
	pushArgs = append(pushArgs, "push", "-f", "origin", "HEAD")
	pushCmd := gitCommand(ctx, pushArgs...)
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %w: %s", err, sanitizeToken(string(output), token))
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/pulls", p.baseURL, owner, repo), bytes.NewReader(payload))
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
		errMsg := githubErrorMessage(resp.Body)
		if strings.Contains(strings.ToLower(errMsg), "a pull request already exists for") {
			prURL, findErr := p.findExistingPR(ctx, owner, repo, head, token)
			if findErr == nil && prURL != "" {
				return prURL, nil
			}
		}
		return "", fmt.Errorf("github create pr returned %s: %s", resp.Status, errMsg)
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.HTMLURL, nil
}

func (p *GitHubProvider) findExistingPR(ctx context.Context, owner, repo, head, token string) (string, error) {
	urlStr := fmt.Sprintf("%s/repos/%s/%s/pulls?head=%s:%s&state=open", p.baseURL, owner, repo, owner, head)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}
	p.authorize(req, token)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		urlStrFallback := fmt.Sprintf("%s/repos/%s/%s/pulls?head=%s&state=open", p.baseURL, owner, repo, head)
		reqFB, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStrFallback, nil)
		if err != nil {
			return "", err
		}
		p.authorize(reqFB, token)
		respFB, err := p.client.Do(reqFB)
		if err != nil {
			return "", err
		}
		defer respFB.Body.Close()
		if respFB.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to list pulls, status: %d", respFB.StatusCode)
		}
		resp = respFB
	}

	var out []struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out) > 0 {
		return out[0].HTMLURL, nil
	}
	return "", fmt.Errorf("no open pull requests found for head %s", head)
}

func (p *GitHubProvider) MergePR(ctx context.Context, owner, repo, prURL, token string) error {
	// Extract pull_number from prURL
	// e.g. https://github.com/owner/repo/pull/123
	parts := strings.Split(prURL, "/")
	if len(parts) == 0 {
		return fmt.Errorf("invalid PR URL: %s", prURL)
	}
	pullNumber := parts[len(parts)-1]

	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%s/merge", p.baseURL, owner, repo, pullNumber)

	payload, err := json.Marshal(map[string]string{
		"merge_method": "merge",
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	p.authorize(req, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github merge pr returned %s: %s", resp.Status, githubErrorMessage(resp.Body))
	}
	return nil
}

func (p *GitHubProvider) ListRepos(ctx context.Context, token string) ([]models.RemoteRepository, error) {
	type ghRepo struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}

	var repos []models.RemoteRepository
	nextURL := p.baseURL + "/user/repos?per_page=100&sort=updated"

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, err
		}
		p.authorize(req, token)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("github list repos returned %s", resp.Status)
		}
		var raw []ghRepo
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, r := range raw {
			repos = append(repos, models.RemoteRepository{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
			})
		}

		nextURL = parseNextLink(resp.Header.Get("Link"))
	}

	return repos, nil
}

// parseNextLink extracts the URL for rel="next" from a GitHub Link header.
func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start != -1 && end != -1 && end > start {
				return part[start+1 : end]
			}
		}
	}
	return ""
}

func githubErrorMessage(body io.Reader) string {
	data, err := io.ReadAll(body)
	if err != nil || len(data) == 0 {
		return ""
	}
	var errBody struct {
		Message string `json:"message"`
		Errors  any    `json:"errors"`
	}
	if err := json.Unmarshal(data, &errBody); err == nil && errBody.Message != "" {
		if errBody.Errors != nil {
			if errorsJSON, err := json.Marshal(errBody.Errors); err == nil {
				return fmt.Sprintf("%s errors=%s", errBody.Message, errorsJSON)
			}
		}
		return errBody.Message
	}
	return strings.TrimSpace(string(data))
}

func (p *GitHubProvider) ValidateToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/user", nil)
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
