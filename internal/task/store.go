package task

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteDBName = "fog.db"

// Store manages task persistence.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore creates a new SQLite-backed task store.
func NewStore(configDir string) (*Store, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	dbPath := filepath.Join(configDir, sqliteDBName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) init() error {
	stmts := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`CREATE TABLE IF NOT EXISTS task_runs (
			id TEXT PRIMARY KEY,
			payload TEXT NOT NULL,
			state TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_task_runs_state ON task_runs(state);`,
		`CREATE INDEX IF NOT EXISTS idx_task_runs_created_at ON task_runs(created_at DESC);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init task store: %w", err)
		}
	}
	return nil
}

// Save persists a task.
func (s *Store) Save(task *Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}
	if task.ID == "" {
		return errors.New("task id is required")
	}

	data, err := task.ToJSON()
	if err != nil {
		return err
	}

	created := task.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	updated := task.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(
		`INSERT INTO task_runs(id, payload, state, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   payload=excluded.payload,
		   state=excluded.state,
		   updated_at=excluded.updated_at`,
		task.ID,
		string(data),
		string(task.State),
		created.Format(time.RFC3339Nano),
		updated.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save task %q: %w", task.ID, err)
	}

	return nil
}

// Get retrieves a task by ID.
func (s *Store) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var payload string
	err := s.db.QueryRow(`SELECT payload FROM task_runs WHERE id = ?`, id).Scan(&payload)
	if err != nil {
		return nil, err
	}
	return FromJSON([]byte(payload))
}

// List returns all tasks.
func (s *Store) List() ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT payload FROM task_runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		task, err := FromJSON([]byte(payload))
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// Delete removes a task.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM task_runs WHERE id = ?`, id)
	return err
}

// ListActive returns non-terminal tasks.
func (s *Store) ListActive() ([]*Task, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	active := make([]*Task, 0, len(all))
	for _, t := range all {
		if !t.IsTerminal() {
			active = append(active, t)
		}
	}
	return active, nil
}
