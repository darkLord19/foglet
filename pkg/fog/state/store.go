package state

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDBName  = "fog.db"
	defaultKeyName = "master.key"
	githubPATKey   = "github_pat"
)

// Store is the Fog state persistence layer backed by SQLite.
type Store struct {
	db  *sql.DB
	key []byte
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
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
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
		time.Now().UTC().Format(time.RFC3339Nano),
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

// SaveGitHubToken encrypts and persists the PAT.
func (s *Store) SaveGitHubToken(token string) error {
	if token == "" {
		return errors.New("token cannot be empty")
	}

	ciphertext, err := encrypt(githubPATKey, []byte(token), s.key)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO secrets(key, ciphertext, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET ciphertext=excluded.ciphertext, updated_at=excluded.updated_at`,
		githubPATKey,
		ciphertext,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save github token: %w", err)
	}

	return nil
}

// GetGitHubToken retrieves and decrypts the PAT.
func (s *Store) GetGitHubToken() (token string, found bool, err error) {
	var ciphertext []byte
	err = s.db.QueryRow(`SELECT ciphertext FROM secrets WHERE key = ?`, githubPATKey).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load github token: %w", err)
	}

	plaintext, err := decrypt(githubPATKey, ciphertext, s.key)
	if err != nil {
		return "", false, fmt.Errorf("decrypt github token: %w", err)
	}

	return string(plaintext), true, nil
}

// HasGitHubToken reports whether an encrypted PAT exists.
func (s *Store) HasGitHubToken() (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM secrets WHERE key = ?`, githubPATKey).Scan(&count); err != nil {
		return false, fmt.Errorf("check github token: %w", err)
	}
	return count > 0, nil
}
