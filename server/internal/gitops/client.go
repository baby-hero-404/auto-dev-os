package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
