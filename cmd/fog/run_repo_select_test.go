package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	foggithub "github.com/darkLord19/foglet/internal/github"
	"github.com/darkLord19/foglet/internal/state"
)

func TestResolveRepoNameForRunUsesFlag(t *testing.T) {
	origTTY := stdinIsTTYFn
	t.Cleanup(func() { stdinIsTTYFn = origTTY })
	stdinIsTTYFn = func() bool {
		t.Fatalf("stdinIsTTYFn should not be called when --repo is provided")
		return false
	}

	got, err := resolveRepoNameForRun(" acme/api ", nil)
	if err != nil {
		t.Fatalf("resolveRepoNameForRun returned error: %v", err)
	}
	if got != "acme/api" {
		t.Fatalf("resolveRepoNameForRun mismatch: got %q want %q", got, "acme/api")
	}
}

func TestResolveRepoNameForRunRequiresRepoWhenNotTTY(t *testing.T) {
	origTTY := stdinIsTTYFn
	t.Cleanup(func() { stdinIsTTYFn = origTTY })
	stdinIsTTYFn = func() bool { return false }

	_, err := resolveRepoNameForRun("", nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--repo is required") {
		t.Fatalf("expected --repo required error, got %v", err)
	}
}

func TestResolveRepoNameForRunPromptsAndSelectsRepo(t *testing.T) {
	fogHome := t.TempDir()
	store, err := state.NewStore(fogHome)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.SaveGitHubToken("token123"); err != nil {
		t.Fatalf("SaveGitHubToken failed: %v", err)
	}

	origTTY := stdinIsTTYFn
	origList := listGitHubReposFn
	origRead := readLineFn
	t.Cleanup(func() {
		stdinIsTTYFn = origTTY
		listGitHubReposFn = origList
		readLineFn = origRead
	})

	stdinIsTTYFn = func() bool { return true }
	listGitHubReposFn = func(token string) ([]foggithub.Repo, error) {
		if token != "token123" {
			t.Fatalf("unexpected token: %q", token)
		}
		return []foggithub.Repo{
			{FullName: "acme/api", OwnerLogin: "acme", Name: "api"},
			{FullName: "acme/web", OwnerLogin: "acme", Name: "web"},
		}, nil
	}
	readLineFn = func(prompt string) (string, error) {
		if !strings.Contains(prompt, "Select repository") {
			t.Fatalf("unexpected prompt: %q", prompt)
		}
		return "2", nil
	}

	got, err := resolveRepoNameForRun("", store)
	if err != nil {
		t.Fatalf("resolveRepoNameForRun returned error: %v", err)
	}
	if got != "acme/web" {
		t.Fatalf("resolveRepoNameForRun mismatch: got %q want %q", got, "acme/web")
	}
}

func TestEnsureRepoRegisteredForRunReturnsExistingRepo(t *testing.T) {
	fogHome := t.TempDir()
	store, err := state.NewStore(fogHome)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	want := state.Repo{
		Name:             "acme/api",
		URL:              "https://github.com/acme/api.git",
		Host:             "github.com",
		Owner:            "acme",
		Repo:             "api",
		BarePath:         filepath.Join(fogHome, "bare.git"),
		BaseWorktreePath: filepath.Join(fogHome, "base"),
		DefaultBranch:    "main",
	}
	if _, err := store.UpsertRepo(want); err != nil {
		t.Fatalf("UpsertRepo failed: %v", err)
	}

	origList := listGitHubReposFn
	t.Cleanup(func() { listGitHubReposFn = origList })
	listGitHubReposFn = func(token string) ([]foggithub.Repo, error) {
		t.Fatalf("listGitHubReposFn should not be called when repo already exists")
		return nil, nil
	}

	got, err := ensureRepoRegisteredForRun("acme/api", store, fogHome)
	if err != nil {
		t.Fatalf("ensureRepoRegisteredForRun returned error: %v", err)
	}
	if got.Name != want.Name {
		t.Fatalf("repo name mismatch: got %q want %q", got.Name, want.Name)
	}
	if got.BaseWorktreePath != want.BaseWorktreePath {
		t.Fatalf("repo base path mismatch: got %q want %q", got.BaseWorktreePath, want.BaseWorktreePath)
	}
}

func TestEnsureRepoRegisteredForRunErrorsWithoutTokenWhenMissing(t *testing.T) {
	fogHome := t.TempDir()
	store, err := state.NewStore(fogHome)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	_, err = ensureRepoRegisteredForRun("acme/api", store, fogHome)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "github token not configured") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestEnsureRepoRegisteredForRunErrorsWhenNotAccessible(t *testing.T) {
	fogHome := t.TempDir()
	store, err := state.NewStore(fogHome)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.SaveGitHubToken("token123"); err != nil {
		t.Fatalf("SaveGitHubToken failed: %v", err)
	}

	origList := listGitHubReposFn
	t.Cleanup(func() { listGitHubReposFn = origList })
	listGitHubReposFn = func(token string) ([]foggithub.Repo, error) {
		return []foggithub.Repo{
			{FullName: "acme/other", OwnerLogin: "acme", Name: "other"},
		}, nil
	}

	_, err = ensureRepoRegisteredForRun("acme/api", store, fogHome)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is not accessible") {
		t.Fatalf("expected not accessible error, got %v", err)
	}
}

func TestEnsureRepoRegisteredForRunAutoImports(t *testing.T) {
	fogHome := t.TempDir()
	store, err := state.NewStore(fogHome)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.SaveGitHubToken("token123"); err != nil {
		t.Fatalf("SaveGitHubToken failed: %v", err)
	}

	origList := listGitHubReposFn
	origRunner := gitRunner
	t.Cleanup(func() {
		listGitHubReposFn = origList
		gitRunner = origRunner
	})

	listGitHubReposFn = func(token string) ([]foggithub.Repo, error) {
		return []foggithub.Repo{
			{
				FullName:      "acme/api",
				Name:          "api",
				OwnerLogin:    "acme",
				CloneURL:      "https://github.com/acme/api.git",
				DefaultBranch: "main",
				Private:       true,
				HTMLURL:       "https://github.com/acme/api",
				ID:            123,
			},
		}, nil
	}

	gitRunner = func(extraEnv []string, args ...string) error {
		_ = extraEnv
		// Simulate bare clone by creating the target directory (last arg).
		if len(args) > 0 && args[0] == "-c" {
			barePath := args[len(args)-1]
			return os.MkdirAll(barePath, 0o755)
		}
		// Simulate base worktree by creating the worktree directory (last arg).
		if len(args) > 0 && args[0] == "--git-dir" {
			basePath := args[len(args)-1]
			return os.MkdirAll(basePath, 0o755)
		}
		return fmt.Errorf("unexpected git args: %v", args)
	}

	repo, err := ensureRepoRegisteredForRun("acme/api", store, fogHome)
	if err != nil {
		t.Fatalf("ensureRepoRegisteredForRun returned error: %v", err)
	}
	if repo.Name != "acme/api" {
		t.Fatalf("repo name mismatch: got %q want %q", repo.Name, "acme/api")
	}
	wantBase := filepath.Join(fogHome, "repos", "acme", "api", "base")
	if repo.BaseWorktreePath != wantBase {
		t.Fatalf("base path mismatch: got %q want %q", repo.BaseWorktreePath, wantBase)
	}
}
