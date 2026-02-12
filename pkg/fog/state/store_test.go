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

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	return store
}
