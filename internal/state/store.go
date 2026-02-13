package state

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDBName      = "fog.db"
	defaultKeyName     = "master.key"
	githubPATKey       = "github_pat"
	settingDefaultTool = "default_tool"
)

// Store is the Fog state persistence layer backed by SQLite.
type Store struct {
	db  *sql.DB
	key []byte
}

// Repo holds Fog's managed repository metadata.
type Repo struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	URL              string    `json:"url"`
	Host             string    `json:"host,omitempty"`
	Owner            string    `json:"owner,omitempty"`
	Repo             string    `json:"repo,omitempty"`
	BarePath         string    `json:"bare_path,omitempty"`
	BaseWorktreePath string    `json:"base_worktree_path"`
	DefaultBranch    string    `json:"default_branch,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
}

// NewStore opens or creates the Fog SQLite database in fogHome.
func NewStore(fogHome string) (*Store, error) {
	if err := os.MkdirAll(fogHome, 0o755); err != nil {
		return nil, fmt.Errorf("create fog home: %w", err)
	}

	keyPath := filepath.Join(fogHome, defaultKeyName)
	key, err := loadOrCreateMasterKey(keyPath)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fogHome, defaultDBName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &Store{db: db, key: key}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) init() error {
	if _, err := s.db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("enable wal: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
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
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			repo_id INTEGER NOT NULL,
			parent_task_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			state TEXT NOT NULL,
			prompt TEXT NOT NULL,
			tool TEXT,
			model TEXT,
			branch TEXT NOT NULL,
			worktree_path TEXT,
			autopr INTEGER NOT NULL DEFAULT 0,
			commit_msg TEXT,
			error TEXT,
			slack_channel_id TEXT,
			slack_thread_ts TEXT,
			slack_root_ts TEXT,
			FOREIGN KEY(repo_id) REFERENCES repos(id)
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
		`CREATE TABLE IF NOT EXISTS task_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			ts TEXT NOT NULL,
			type TEXT NOT NULL,
			message TEXT,
			data TEXT,
			FOREIGN KEY(task_id) REFERENCES tasks(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_repo_created ON tasks(repo_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_task_events_task_ts ON task_events(task_id, ts DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_repo_updated ON sessions(repo_name, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_session_created ON runs(session_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_run_events_run_ts ON run_events(run_id, ts DESC);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	if err := s.ensureRunsSchema(); err != nil {
		return err
	}

	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SetSetting stores a Fog setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
	 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key,
		value,
		nowRFC3339Nano(),
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// GetSetting returns a setting by key. found=false when missing.
func (s *Store) GetSetting(key string) (value string, found bool, err error) {
	err = s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == nil {
		return value, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("get setting %q: %w", key, err)
}

// SaveSecret encrypts and persists a secret value by key.
func (s *Store) SaveSecret(key, value string) error {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return errors.New("secret key cannot be empty")
	}
	if value == "" {
		return errors.New("secret value cannot be empty")
	}
	ciphertext, err := encrypt(key, []byte(value), s.key)
	if err != nil {
		return fmt.Errorf("encrypt secret %q: %w", key, err)
	}
	_, err = s.db.Exec(
		`INSERT INTO secrets(key, ciphertext, updated_at) VALUES(?, ?, ?)
	 ON CONFLICT(key) DO UPDATE SET ciphertext=excluded.ciphertext, updated_at=excluded.updated_at`,
		key,
		ciphertext,
		nowRFC3339Nano(),
	)
	if err != nil {
		return fmt.Errorf("save secret %q: %w", key, err)
	}
	return nil
}

// GetSecret retrieves and decrypts a secret by key.
func (s *Store) GetSecret(key string) (value string, found bool, err error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, errors.New("secret key cannot be empty")
	}
	var ciphertext []byte
	err = s.db.QueryRow(`SELECT ciphertext FROM secrets WHERE key = ?`, key).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load secret %q: %w", key, err)
	}

	plaintext, err := decrypt(key, ciphertext, s.key)
	if err != nil {
		return "", false, fmt.Errorf("decrypt secret %q: %w", key, err)
	}

	return string(plaintext), true, nil
}

// HasSecret reports whether a secret exists.
func (s *Store) HasSecret(key string) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return false, errors.New("secret key cannot be empty")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM secrets WHERE key = ?`, key).Scan(&count); err != nil {
		return false, fmt.Errorf("check secret %q: %w", key, err)
	}
	return count > 0, nil
}

// DeleteSecret removes one secret key.
func (s *Store) DeleteSecret(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("secret key cannot be empty")
	}
	if _, err := s.db.Exec(`DELETE FROM secrets WHERE key = ?`, key); err != nil {
		return fmt.Errorf("delete secret %q: %w", key, err)
	}
	return nil
}

// SaveGitHubToken encrypts and persists the PAT.
func (s *Store) SaveGitHubToken(token string) error {
	return s.SaveSecret(githubPATKey, token)
}

// GetGitHubToken retrieves and decrypts the PAT.
func (s *Store) GetGitHubToken() (token string, found bool, err error) {
	return s.GetSecret(githubPATKey)
}

// HasGitHubToken reports whether an encrypted PAT exists.
func (s *Store) HasGitHubToken() (bool, error) {
	return s.HasSecret(githubPATKey)
}

// SetDefaultTool stores the default AI tool used when task input omits "tool".
func (s *Store) SetDefaultTool(tool string) error {
	if tool == "" {
		return errors.New("default tool cannot be empty")
	}
	return s.SetSetting(settingDefaultTool, tool)
}

// GetDefaultTool returns the configured default AI tool.
func (s *Store) GetDefaultTool() (tool string, found bool, err error) {
	return s.GetSetting(settingDefaultTool)
}

// UpsertRepo inserts or updates a managed repository by name.
func (s *Store) UpsertRepo(repo Repo) (int64, error) {
	if repo.Name == "" {
		return 0, errors.New("repo name cannot be empty")
	}
	if repo.URL == "" {
		return 0, errors.New("repo url cannot be empty")
	}
	if repo.Host == "" {
		return 0, errors.New("repo host cannot be empty")
	}
	if repo.BarePath == "" {
		return 0, errors.New("repo bare path cannot be empty")
	}
	if repo.BaseWorktreePath == "" {
		return 0, errors.New("repo base worktree path cannot be empty")
	}

	now := nowRFC3339Nano()
	_, err := s.db.Exec(
		`INSERT INTO repos(name, url, host, owner, repo, bare_path, base_worktree_path, default_branch, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   url=excluded.url,
		   host=excluded.host,
		   owner=excluded.owner,
		   repo=excluded.repo,
		   bare_path=excluded.bare_path,
		   base_worktree_path=excluded.base_worktree_path,
		   default_branch=excluded.default_branch`,
		repo.Name,
		repo.URL,
		repo.Host,
		repo.Owner,
		repo.Repo,
		repo.BarePath,
		repo.BaseWorktreePath,
		repo.DefaultBranch,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("upsert repo %q: %w", repo.Name, err)
	}

	var id int64
	if err := s.db.QueryRow(`SELECT id FROM repos WHERE name = ?`, repo.Name).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup repo %q: %w", repo.Name, err)
	}
	return id, nil
}

// ListRepos returns all managed repositories ordered by name.
func (s *Store) ListRepos() ([]Repo, error) {
	rows, err := s.db.Query(
		`SELECT id, name, url, host, owner, repo, bare_path, base_worktree_path, default_branch, created_at
		   FROM repos
		  ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()

	repos := make([]Repo, 0)
	for rows.Next() {
		var repo Repo
		var createdAt string
		if err := rows.Scan(
			&repo.ID,
			&repo.Name,
			&repo.URL,
			&repo.Host,
			&repo.Owner,
			&repo.Repo,
			&repo.BarePath,
			&repo.BaseWorktreePath,
			&repo.DefaultBranch,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		if ts, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			repo.CreatedAt = ts
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repos: %w", err)
	}
	return repos, nil
}

// GetRepoByName returns one managed repo by alias.
func (s *Store) GetRepoByName(name string) (Repo, bool, error) {
	var repo Repo
	var createdAt string
	err := s.db.QueryRow(
		`SELECT id, name, url, host, owner, repo, bare_path, base_worktree_path, default_branch, created_at
		   FROM repos
		  WHERE name = ?`,
		name,
	).Scan(
		&repo.ID,
		&repo.Name,
		&repo.URL,
		&repo.Host,
		&repo.Owner,
		&repo.Repo,
		&repo.BarePath,
		&repo.BaseWorktreePath,
		&repo.DefaultBranch,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Repo{}, false, nil
	}
	if err != nil {
		return Repo{}, false, fmt.Errorf("get repo %q: %w", name, err)
	}

	if ts, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		repo.CreatedAt = ts
	}
	return repo, true, nil
}

func nowRFC3339Nano() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func (s *Store) ensureRunsSchema() error {
	const table = "runs"
	if hasWorktree, err := s.tableColumnExists(table, "worktree_path"); err != nil {
		return err
	} else if !hasWorktree {
		if _, err := s.db.Exec(`ALTER TABLE runs ADD COLUMN worktree_path TEXT`); err != nil {
			return fmt.Errorf("add runs.worktree_path column: %w", err)
		}
	}
	return nil
}

func (s *Store) tableColumnExists(tableName, columnName string) (bool, error) {
	tableName = strings.TrimSpace(tableName)
	columnName = strings.TrimSpace(columnName)
	if tableName == "" || columnName == "" {
		return false, errors.New("table and column names are required")
	}

	rows, err := s.db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, fmt.Errorf("table info for %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan table info %s: %w", tableName, err)
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table info %s: %w", tableName, err)
	}
	return false, nil
}
