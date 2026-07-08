package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

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
