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

func TestStoreSecretRoundTripAndDelete(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SaveSecret("cloud_device_token", "secret-token"); err != nil {
		t.Fatalf("save secret failed: %v", err)
	}
	got, found, err := store.GetSecret("cloud_device_token")
	if err != nil {
		t.Fatalf("get secret failed: %v", err)
	}
	if !found || got != "secret-token" {
		t.Fatalf("unexpected secret value: found=%v got=%q", found, got)
	}

	has, err := store.HasSecret("cloud_device_token")
	if err != nil {
		t.Fatalf("has secret failed: %v", err)
	}
	if !has {
		t.Fatal("expected secret to exist")
	}

	if err := store.DeleteSecret("cloud_device_token"); err != nil {
		t.Fatalf("delete secret failed: %v", err)
	}
	_, found, err = store.GetSecret("cloud_device_token")
	if err != nil {
		t.Fatalf("get secret after delete failed: %v", err)
	}
	if found {
		t.Fatal("expected secret to be deleted")
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

func TestGetRepoByName(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	_, err := store.UpsertRepo(Repo{
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

	repo, found, err := store.GetRepoByName("acme-api")
	if err != nil {
		t.Fatalf("GetRepoByName failed: %v", err)
	}
	if !found {
		t.Fatal("expected repo to exist")
	}
	if repo.Repo != "api" {
		t.Fatalf("unexpected repo payload: %+v", repo)
	}

	_, found, err = store.GetRepoByName("missing")
	if err != nil {
		t.Fatalf("GetRepoByName missing failed: %v", err)
	}
	if found {
		t.Fatal("expected missing repo to not be found")
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

// TestStoreInitMigratesLegacyTasksTable reproduces the behaviour when an
// on-disk database holds the pre-Kanban `tasks` table: migration 1 must drop
// the incompatible table and recreate it rather than failing to build the
// status index.
func TestStoreInitMigratesLegacyTasksTable(t *testing.T) {
	home := t.TempDir()

	store, err := NewStore(home)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	// Replace the current tasks table with the legacy shape (no status column).
	if _, err := store.db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks failed: %v", err)
	}
	if _, err := store.db.Exec(`CREATE TABLE tasks (
		id TEXT PRIMARY KEY,
		repo_id INTEGER NOT NULL,
		state TEXT NOT NULL,
		prompt TEXT NOT NULL,
		branch TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy tasks failed: %v", err)
	}
	// Reset user_version so migrate() re-runs migration 1 on the next open.
	// This simulates a database created before the migration system existed.
	if _, err := store.db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatalf("reset user_version: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store failed: %v", err)
	}

	// Re-opening triggers migrate() against the legacy table with user_version=0.
	store, err = NewStore(home)
	if err != nil {
		t.Fatalf("reopen store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	hasStatus, err := store.tableColumnExists("tasks", "status")
	if err != nil {
		t.Fatalf("column check failed: %v", err)
	}
	if !hasStatus {
		t.Fatal("expected tasks table to be recreated with a status column")
	}
}

func TestMigrateAppliesVersions(t *testing.T) {
	home := t.TempDir()
	store, err := NewStore(home)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	var ver int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	want := len(allMigrations)
	if ver != want {
		t.Fatalf("user_version = %d, want %d", ver, want)
	}

	for _, table := range []string{"settings", "secrets", "repos", "sessions", "runs", "run_events", "tasks"} {
		var name string
		if err := store.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	home := t.TempDir()
	store, err := NewStore(home)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	store.Close()

	store, err = NewStore(home)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer store.Close()

	var ver int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if ver != len(allMigrations) {
		t.Fatalf("user_version = %d after second open, want %d", ver, len(allMigrations))
	}
}

func TestMigrateBackupCreated(t *testing.T) {
	home := t.TempDir()
	store, err := NewStore(home)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	// Reset so re-opening will run migrations (and create a backup for version 0).
	if _, err := store.db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatalf("reset version: %v", err)
	}
	store.Close()

	store, err = NewStore(home)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer store.Close()

	bak := filepath.Join(home, "fog.db.bak-0")
	if _, err := os.Stat(bak); os.IsNotExist(err) {
		t.Fatalf("expected backup %s to exist", bak)
	}
}
