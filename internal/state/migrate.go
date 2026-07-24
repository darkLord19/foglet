package state

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type migration struct {
	version int
	name    string
	up      func(tx *sql.Tx) error
}

// allMigrations is the ordered list of schema versions.
// Each runs once, in its own transaction; user_version is stamped inside
// the same transaction so a partial run can never be re-entered.
var allMigrations = []migration{
	{1, "initial_schema", migration1InitialSchema},
	{2, "schema_fixups", migration2SchemaFixups},
	{3, "run_lifecycle_columns", migration3RunLifecycleColumns},
}

// migration1InitialSchema establishes the full baseline schema.
// All statements are CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS
// so they are safe against a non-empty database.
func migration1InitialSchema(tx *sql.Tx) error {
	if err := dropLegacyTasksTableTx(tx); err != nil {
		return err
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS secrets (
			key TEXT PRIMARY KEY,
			ciphertext BLOB NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS repos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			url TEXT NOT NULL,
			host TEXT NOT NULL,
			owner TEXT,
			repo TEXT,
			bare_path TEXT NOT NULL,
			base_worktree_path TEXT NOT NULL,
			default_branch TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			repo_name TEXT NOT NULL,
			branch TEXT NOT NULL,
			worktree_path TEXT NOT NULL,
			tool TEXT NOT NULL,
			model TEXT,
			autopr INTEGER NOT NULL DEFAULT 0,
			pr_url TEXT,
			status TEXT NOT NULL,
			busy INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(repo_name) REFERENCES repos(name)
		);`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			worktree_path TEXT,
			state TEXT NOT NULL,
			commit_sha TEXT,
			commit_msg TEXT,
			error TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT,
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS run_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			ts TEXT NOT NULL,
			type TEXT NOT NULL,
			message TEXT,
			data TEXT,
			FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			body TEXT,
			status TEXT NOT NULL,
			position REAL NOT NULL,
			repo_name TEXT,
			tool TEXT,
			model TEXT,
			base_branch TEXT,
			session_id TEXT,
			provider TEXT NOT NULL DEFAULT 'local',
			external_id TEXT,
			external_key TEXT,
			external_url TEXT,
			external_status TEXT,
			synced_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			trashed_at TEXT,
			FOREIGN KEY(repo_name) REFERENCES repos(name),
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_repo_updated ON sessions(repo_name, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_session_created ON runs(session_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_run_events_run_ts ON run_events(run_id, ts DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status_position ON tasks(status, position);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_provider_external
			ON tasks(provider, external_id)
			WHERE external_id IS NOT NULL;`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// migration2SchemaFixups backfills columns added to existing tables after
// the initial schema shipped. For databases created fresh by migration 1
// these columns already exist and the ALTER TABLE calls are skipped.
func migration2SchemaFixups(tx *sql.Tx) error {
	if ok, err := txColumnExists(tx, "runs", "worktree_path"); err != nil {
		return err
	} else if !ok {
		if _, err := tx.Exec(`ALTER TABLE runs ADD COLUMN worktree_path TEXT`); err != nil {
			return fmt.Errorf("add runs.worktree_path: %w", err)
		}
	}
	if ok, err := txColumnExists(tx, "tasks", "trashed_at"); err != nil {
		return err
	} else if !ok {
		if _, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN trashed_at TEXT`); err != nil {
			return fmt.Errorf("add tasks.trashed_at: %w", err)
		}
	}
	return nil
}

// migration3RunLifecycleColumns adds columns used by crash recovery (F2)
// and the scheduler (F1).
func migration3RunLifecycleColumns(tx *sql.Tx) error {
	cols := []struct {
		col string
		ddl string
	}{
		{"pid", `ALTER TABLE runs ADD COLUMN pid INTEGER`},
		{"pgid", `ALTER TABLE runs ADD COLUMN pgid INTEGER`},
		{"heartbeat_at", `ALTER TABLE runs ADD COLUMN heartbeat_at TEXT`},
	}
	for _, c := range cols {
		if ok, err := txColumnExists(tx, "runs", c.col); err != nil {
			return err
		} else if !ok {
			if _, err := tx.Exec(c.ddl); err != nil {
				return fmt.Errorf("add runs.%s: %w", c.col, err)
			}
		}
	}
	return nil
}

// migrate applies any pending migrations in order, each in its own
// transaction. user_version is set inside the transaction so a crash
// mid-migration leaves the database at the last committed version.
func migrate(db *sql.DB, fogHome string) error {
	var currentVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	for _, m := range allMigrations {
		if m.version <= currentVersion {
			continue
		}
		if err := backupDB(fogHome, currentVersion); err != nil {
			return fmt.Errorf("backup before migration %d: %w", m.version, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}
		if err := m.up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", m.version, m.name, err)
		}
		// user_version cannot be set with a parameterised query; the integer
		// is compile-time-safe here.
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set user_version %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
		currentVersion = m.version
	}
	return nil
}

// backupDB copies fog.db to fog.db.bak-<version> before upgrading.
// The destination is overwritten on each upgrade — it is a one-step undo,
// not a history.
func backupDB(fogHome string, version int) error {
	src := filepath.Join(fogHome, defaultDBName)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	dst := filepath.Join(fogHome, fmt.Sprintf("fog.db.bak-%d", version))
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return out.Sync()
}

// dropLegacyTasksTableTx drops the tasks table when it exists in the
// pre-Kanban shape (no status column). The current shape is recreated
// by the CREATE TABLE IF NOT EXISTS statement that follows.
func dropLegacyTasksTableTx(tx *sql.Tx) error {
	var name string
	err := tx.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='tasks'`,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check tasks table: %w", err)
	}
	hasStatus, err := txColumnExists(tx, "tasks", "status")
	if err != nil {
		return err
	}
	if hasStatus {
		return nil
	}
	if _, err := tx.Exec(`DROP TABLE tasks`); err != nil {
		return fmt.Errorf("drop legacy tasks: %w", err)
	}
	return nil
}

// txColumnExists reports whether tableName has a column named columnName,
// queried through an active transaction. PRAGMA table_info is read-only
// and is safe inside a write transaction.
func txColumnExists(tx *sql.Tx, tableName, columnName string) (bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, fmt.Errorf("table_info(%s): %w", tableName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var col, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &col, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan table_info(%s): %w", tableName, err)
		}
		if strings.EqualFold(col, columnName) {
			return true, nil
		}
	}
	return false, rows.Err()
}
