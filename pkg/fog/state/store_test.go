package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSettingsRoundTrip(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SetSetting("default_tool", "cursor"); err != nil {
		t.Fatalf("set setting failed: %v", err)
	}

	got, found, err := store.GetSetting("default_tool")
	if err != nil {
		t.Fatalf("get setting failed: %v", err)
	}
	if !found {
		t.Fatal("expected setting to exist")
	}
	if got != "cursor" {
		t.Fatalf("setting mismatch: got=%q want=%q", got, "cursor")
	}
}

func TestStoreGitHubTokenEncryptedAndOverwrite(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	first := "ghp_first_token"
	if err := store.SaveGitHubToken(first); err != nil {
		t.Fatalf("save token failed: %v", err)
	}

	got, found, err := store.GetGitHubToken()
	if err != nil {
		t.Fatalf("get token failed: %v", err)
	}
	if !found || got != first {
		t.Fatalf("token mismatch: found=%v got=%q want=%q", found, got, first)
	}

	second := "ghp_second_token"
	if err := store.SaveGitHubToken(second); err != nil {
		t.Fatalf("overwrite token failed: %v", err)
	}

	got, found, err = store.GetGitHubToken()
	if err != nil {
		t.Fatalf("get token after overwrite failed: %v", err)
	}
	if !found || got != second {
		t.Fatalf("token overwrite mismatch: found=%v got=%q want=%q", found, got, second)
	}

	dbBytes, err := os.ReadFile(filepath.Join(tmp, defaultDBName))
	if err != nil {
		t.Fatalf("read db failed: %v", err)
	}
	if strings.Contains(string(dbBytes), second) {
		t.Fatal("raw token should not appear in sqlite file")
	}
}

func TestHasGitHubToken(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	has, err := store.HasGitHubToken()
	if err != nil {
		t.Fatalf("has token failed: %v", err)
	}
	if has {
		t.Fatal("expected no token initially")
	}

	if err := store.SaveGitHubToken("ghp_token"); err != nil {
		t.Fatalf("save token failed: %v", err)
	}

	has, err = store.HasGitHubToken()
	if err != nil {
		t.Fatalf("has token failed: %v", err)
	}
	if !has {
		t.Fatal("expected token after save")
	}
}

func TestDefaultTool(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SetDefaultTool("cursor"); err != nil {
		t.Fatalf("set default tool failed: %v", err)
	}

	tool, found, err := store.GetDefaultTool()
	if err != nil {
		t.Fatalf("get default tool failed: %v", err)
	}
	if !found {
		t.Fatal("expected default tool")
	}
	if tool != "cursor" {
		t.Fatalf("default tool mismatch: got=%q want=%q", tool, "cursor")
	}
}

func TestUpsertAndListRepos(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	id, err := store.UpsertRepo(Repo{
		Name:             "acme-api",
		URL:              "https://github.com/acme/api.git",
		Host:             "github.com",
		Owner:            "acme",
		Repo:             "api",
		BarePath:         "/tmp/acme-api/repo.git",
		BaseWorktreePath: "/tmp/acme-api/base",
		DefaultBranch:    "main",
	})
	if err != nil {
		t.Fatalf("upsert repo failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive repo id, got %d", id)
	}

	updatedID, err := store.UpsertRepo(Repo{
		Name:             "acme-api",
		URL:              "https://github.com/acme/api.git",
		Host:             "github.com",
		Owner:            "acme",
		Repo:             "api",
		BarePath:         "/tmp/acme-api/repo.git",
		BaseWorktreePath: "/tmp/acme-api/base",
		DefaultBranch:    "develop",
	})
	if err != nil {
		t.Fatalf("upsert repo update failed: %v", err)
	}
	if updatedID != id {
		t.Fatalf("expected same id after upsert, got %d want %d", updatedID, id)
	}

	repos, err := store.ListRepos()
	if err != nil {
		t.Fatalf("list repos failed: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].DefaultBranch != "develop" {
		t.Fatalf("expected updated default branch, got %q", repos[0].DefaultBranch)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	return store
}
