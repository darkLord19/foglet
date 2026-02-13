package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultAPIBase = "https://api.github.com"

// Client wraps GitHub REST API calls needed by Fog onboarding.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// Repo represents a GitHub repository visible to the configured token.
type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	OwnerLogin    string `json:"owner_login,omitempty"`
	HTMLURL       string `json:"html_url,omitempty"`
}

// NewClient builds a client against the default GitHub API URL.
func NewClient(token string) *Client {
	return NewClientWithBaseURL(token, defaultAPIBase, http.DefaultClient)
}

// NewClientWithBaseURL is exposed for tests and enterprise-hosted endpoints.
func NewClientWithBaseURL(token, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		token:      strings.TrimSpace(token),
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// ValidateToken checks token validity by calling GET /user.
func (c *Client) ValidateToken(ctx context.Context) error {
	if c.token == "" {
		return errors.New("token is required")
	}

	resp, err := c.doJSON(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New("github token is invalid or missing required scopes")
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("github token validation failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// ListRepos returns all repositories visible to the token.
func (c *Client) ListRepos(ctx context.Context) ([]Repo, error) {
	if c.token == "" {
		return nil, errors.New("token is required")
	}

	all := make([]Repo, 0)
	page := 1
	for {
		path := fmt.Sprintf("/user/repos?per_page=100&page=%d&sort=updated", page)
		resp, err := c.doJSON(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			_ = resp.Body.Close()
			return nil, errors.New("github token is invalid or missing required scopes")
		}
		if resp.StatusCode/100 != 2 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github list repos failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		pageRepos, err := decodeRepos(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		all = append(all, pageRepos...)

		if !hasNextPage(resp.Header.Get("Link")) {
			break
		}
		page++
	}

	return all, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fogd")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github api: %w", err)
	}
	return resp, nil
}

func decodeRepos(r io.Reader) ([]Repo, error) {
	var raw []struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
		HTMLURL       string `json:"html_url"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}

	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode repos: %w", err)
	}

	repos := make([]Repo, 0, len(raw))
	for _, item := range raw {
		repos = append(repos, Repo{
			ID:            item.ID,
			Name:          item.Name,
			FullName:      item.FullName,
			CloneURL:      item.CloneURL,
			Private:       item.Private,
			DefaultBranch: item.DefaultBranch,
			OwnerLogin:    item.Owner.Login,
			HTMLURL:       item.HTMLURL,
		})
	}
	return repos, nil
}

func hasNextPage(linkHeader string) bool {
	if linkHeader == "" {
		return false
	}
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			return true
		}
	}
	return false
}
