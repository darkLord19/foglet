package github

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestValidateToken(t *testing.T) {
	client := newTestClient("good-token", func(req *http.Request) *http.Response {
		if req.URL.Path != "/user" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		if token != "good-token" {
			return jsonResponse(http.StatusUnauthorized, `{"message":"bad credentials"}`)
		}
		return jsonResponse(http.StatusOK, `{"login":"octocat"}`)
	})

	if err := client.ValidateToken(context.Background()); err != nil {
		t.Fatalf("validate token should pass: %v", err)
	}

	bad := newTestClient("bad-token", func(req *http.Request) *http.Response {
		_ = req
		return jsonResponse(http.StatusUnauthorized, `{"message":"bad credentials"}`)
	})
	if err := bad.ValidateToken(context.Background()); err == nil {
		t.Fatal("expected token validation failure")
	}
}

func TestListReposPaginated(t *testing.T) {
	client := newTestClient("good-token", func(req *http.Request) *http.Response {
		if req.URL.Path != "/user/repos" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Fatal("missing bearer token")
		}

		page := req.URL.Query().Get("page")
		if page == "" {
			page = "1"
		}

		if page == "1" {
			headers := map[string]string{
				"Link": "<https://api.github.local/user/repos?per_page=100&page=2>; rel=\"next\"",
			}
			return repoResponse(headers, []Repo{{ID: 1, Name: "repo-one", FullName: "acme/repo-one", CloneURL: "https://github.com/acme/repo-one.git", OwnerLogin: "acme", DefaultBranch: "main"}})
		}
		if page == "2" {
			return repoResponse(nil, []Repo{{ID: 2, Name: "repo-two", FullName: "acme/repo-two", CloneURL: "https://github.com/acme/repo-two.git", OwnerLogin: "acme", DefaultBranch: "main"}})
		}
		t.Fatalf("unexpected page: %s", page)
		return nil
	})

	repos, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("list repos failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("unexpected repo count: got %d want 2", len(repos))
	}
	if repos[0].Name != "repo-one" || repos[1].Name != "repo-two" {
		t.Fatalf("unexpected repo order: %+v", repos)
	}
}

func TestListReposUnauthorized(t *testing.T) {
	client := newTestClient("bad-token", func(req *http.Request) *http.Response {
		_ = req
		return jsonResponse(http.StatusUnauthorized, `{"message":"bad credentials"}`)
	})
	if _, err := client.ListRepos(context.Background()); err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func newTestClient(token string, fn roundTripFunc) *Client {
	httpClient := &http.Client{Transport: fn}
	return NewClientWithBaseURL(token, "https://api.github.local", httpClient)
}

type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func jsonResponse(status int, body string) *http.Response {
	res := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
	res.Header.Set("Content-Type", "application/json")
	res.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return res
}

func repoResponse(headers map[string]string, repos []Repo) *http.Response {
	payload := make([]string, 0, len(repos))
	for _, repo := range repos {
		payload = append(payload,
			`{"id":`+strconv.FormatInt(repo.ID, 10)+
				`,"name":"`+repo.Name+`"`+
				`,"full_name":"`+repo.FullName+`"`+
				`,"clone_url":"`+repo.CloneURL+`"`+
				`,"private":false`+
				`,"default_branch":"`+repo.DefaultBranch+`"`+
				`,"html_url":"https://github.com/`+repo.FullName+`"`+
				`,"owner":{"login":"`+repo.OwnerLogin+`"}}`)
	}
	body := "[" + strings.Join(payload, ",") + "]"
	res := jsonResponse(http.StatusOK, body)
	for k, v := range headers {
		res.Header.Set(k, v)
	}
	return res
}
