package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	foggithub "github.com/darkLord19/wtx/internal/github"
)

func TestParseIndex(t *testing.T) {
	idx, err := parseIndex("2", 3)
	if err != nil {
		t.Fatalf("parseIndex returned error: %v", err)
	}
	if idx != 1 {
		t.Fatalf("parseIndex mismatch: got %d want 1", idx)
	}
}

func TestParseIndexesDedup(t *testing.T) {
	indexes, err := parseIndexes("1, 2, 2, 3", 3)
	if err != nil {
		t.Fatalf("parseIndexes returned error: %v", err)
	}
	want := []int{0, 1, 2}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("parseIndexes mismatch: got %v want %v", indexes, want)
	}
}

func TestRepoAlias(t *testing.T) {
	repo := foggithub.Repo{FullName: "acme/api", OwnerLogin: "acme", Name: "api"}
	got := repoAlias(repo)
	if got != "acme-api" {
		t.Fatalf("repoAlias mismatch: got %q want %q", got, "acme-api")
	}
}

func TestSelectReposByFullName(t *testing.T) {
	repos := []foggithub.Repo{
		{FullName: "acme/api", Name: "api", OwnerLogin: "acme"},
		{FullName: "acme/web", Name: "web", OwnerLogin: "acme"},
	}
	selected, err := selectRepos(repos, "acme/web")
	if err != nil {
		t.Fatalf("selectRepos returned error: %v", err)
	}
	if len(selected) != 1 || selected[0].FullName != "acme/web" {
		t.Fatalf("unexpected selected repos: %+v", selected)
	}
}

func TestEnsureBareRepoInitialized(t *testing.T) {
	tmp := t.TempDir()
	barePath := filepath.Join(tmp, "repo.git")
	basePath := filepath.Join(tmp, "base")

	repo := foggithub.Repo{
		FullName: "acme/api",
		CloneURL: "https://github.com/acme/api.git",
	}

	calls := make([][]string, 0, 2)
	origRunner := gitRunner
	t.Cleanup(func() { gitRunner = origRunner })

	gitRunner = func(extraEnv []string, args ...string) error {
		_ = extraEnv
		calls = append(calls, append([]string(nil), args...))
		if len(args) >= 1 && args[0] == "-c" {
			if err := os.MkdirAll(barePath, 0o755); err != nil {
				return err
			}
		}
		if len(args) >= 4 && args[0] == "--git-dir" {
			if err := os.MkdirAll(basePath, 0o755); err != nil {
				return err
			}
		}
		return nil
	}

	if err := ensureBareRepoInitialized("token", repo, barePath, basePath); err != nil {
		t.Fatalf("ensureBareRepoInitialized failed: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 git calls, got %d", len(calls))
	}
}

func TestCloneBareRepoWithTokenFallsBackToBasic(t *testing.T) {
	origRunner := gitRunner
	t.Cleanup(func() { gitRunner = origRunner })

	calls := make([][]string, 0, 2)
	gitRunner = func(extraEnv []string, args ...string) error {
		_ = extraEnv
		calls = append(calls, append([]string(nil), args...))
		if len(calls) == 1 {
			return fmt.Errorf("auth failed")
		}
		return nil
	}

	err := cloneBareRepoWithToken("token123", "https://github.com/acme/api.git", filepath.Join(t.TempDir(), "repo.git"))
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 git clone attempts, got %d", len(calls))
	}
	if !strings.Contains(calls[0][1], "Authorization: Bearer token123") {
		t.Fatalf("first clone should use bearer header, got args: %v", calls[0])
	}
	if !strings.Contains(calls[1][1], "Authorization: Basic") {
		t.Fatalf("second clone should use basic header, got args: %v", calls[1])
	}
}

func TestBasicAuthCredential(t *testing.T) {
	got := basicAuthCredential("abc123")
	want := base64.StdEncoding.EncodeToString([]byte("x-access-token:abc123"))
	if got != want {
		t.Fatalf("basicAuthCredential mismatch: got %q want %q", got, want)
	}
}

func TestSanitizeGitArgs(t *testing.T) {
	args := []string{
		"-c",
		"http.extraHeader=Authorization: Bearer super-secret-token",
		"clone",
	}
	sanitized := sanitizeGitArgs(args)
	if strings.Contains(strings.Join(sanitized, " "), "super-secret-token") {
		t.Fatalf("expected token to be sanitized, got args: %v", sanitized)
	}
}
