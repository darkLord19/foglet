package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Task is a unit of work on the board. It exists before any agent runs, and
// links to a Session once work starts. See internal/task for the domain rules
// that govern status changes.
type Task struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Body     string  `json:"body,omitempty"`
	Status   string  `json:"status"`
	Position float64 `json:"position"`

	// How to run it. Empty fields fall back to the global defaults at start time.
	RepoName   string `json:"repo_name,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Model      string `json:"model,omitempty"`
	BaseBranch string `json:"base_branch,omitempty"`

	// SessionID is set once work has been started at least once.
	SessionID string `json:"session_id,omitempty"`

	// Mirror of the upstream issue. Empty for provider='local'.
	Provider       string     `json:"provider"`
	ExternalID     string     `json:"external_id,omitempty"`
	ExternalKey    string     `json:"external_key,omitempty"`
	ExternalURL    string     `json:"external_url,omitempty"`
	ExternalStatus string     `json:"external_status,omitempty"`
	SyncedAt       *time.Time `json:"synced_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ErrNotFound is the package's single not-found signal.
//
// Lookups that may legitimately miss return (T, bool, error) and do not use it.
// Operations addressed at a specific id — mutations, and gets that treat a
// missing row as an error — wrap it, so callers can errors.Is rather than
// matching on message text. internal/api used to do the latter:
//
//	if strings.Contains(strings.ToLower(err.Error()), "not found") { ... }
var ErrNotFound = errors.New("not found")

// ErrTaskNotFound is returned when a task id does not resolve. It wraps
// ErrNotFound, so errors.Is matches either.
var ErrTaskNotFound = fmt.Errorf("%w: task", ErrNotFound)

// positionGap is the spacing between adjacent cards when a column is built
// from scratch. Cards inserted between two neighbours take the midpoint, so a
// wide initial gap keeps float precision comfortable across many reorders.
const positionGap = 1024.0

const taskColumns = `id, title, body, status, position, repo_name, tool, model,
	base_branch, session_id, provider, external_id, external_key, external_url,
	external_status, synced_at, created_at, updated_at`

// CreateTask inserts a task, placing it at the end of its column when no
// position is supplied.
func (s *Store) CreateTask(t Task) (Task, error) {
	t.ID = strings.TrimSpace(t.ID)
	t.Title = strings.TrimSpace(t.Title)
	t.Status = strings.TrimSpace(t.Status)
	t.Provider = strings.TrimSpace(t.Provider)

	if t.Provider == "" {
		t.Provider = "local"
	}

	switch {
	case t.ID == "":
		return Task{}, errors.New("task id cannot be empty")
	case t.Title == "":
		return Task{}, errors.New("task title cannot be empty")
	case t.Status == "":
		return Task{}, errors.New("task status cannot be empty")
	}

	if t.Position == 0 {
		end, err := s.endPosition(t.Status)
		if err != nil {
			return Task{}, err
		}
		t.Position = end
	}

	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.CreatedAt
	}

	_, err := s.db.Exec(
		`INSERT INTO tasks(`+taskColumns+`)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, nullIfEmpty(t.Body), t.Status, t.Position,
		nullIfEmpty(t.RepoName), nullIfEmpty(t.Tool), nullIfEmpty(t.Model),
		nullIfEmpty(t.BaseBranch), nullIfEmpty(t.SessionID), t.Provider,
		nullIfEmpty(t.ExternalID), nullIfEmpty(t.ExternalKey),
		nullIfEmpty(t.ExternalURL), nullIfEmpty(t.ExternalStatus),
		nullTime(t.SyncedAt),
		t.CreatedAt.Format(time.RFC3339Nano),
		t.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

// ListTasks returns every task ordered by column then position, which is the
// order the board renders.
func (s *Store) ListTasks() ([]Task, error) {
	rows, err := s.db.Query(`SELECT ` + taskColumns + ` FROM tasks ORDER BY status, position`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0, 32)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTask fetches one task by id.
func (s *Store) GetTask(id string) (Task, error) {
	row := s.db.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, strings.TrimSpace(id))
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return t, err
}

// UpdateTask persists the editable fields of a task. It does not move the task
// between columns — use MoveTask, which carries the transition rules.
func (s *Store) UpdateTask(t Task) error {
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" {
		return errors.New("task title cannot be empty")
	}

	res, err := s.db.Exec(
		`UPDATE tasks SET title = ?, body = ?, repo_name = ?, tool = ?, model = ?,
		    base_branch = ?, updated_at = ?
		 WHERE id = ?`,
		t.Title, nullIfEmpty(t.Body), nullIfEmpty(t.RepoName), nullIfEmpty(t.Tool),
		nullIfEmpty(t.Model), nullIfEmpty(t.BaseBranch), nowRFC3339Nano(), t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return requireOneRow(res, ErrTaskNotFound)
}

// MoveTask places a task in a column at a given index, computing a fractional
// position between its new neighbours.
//
// Callers are responsible for validating the transition (internal/task) and for
// deciding whether the move starts an agent. This function only records where
// the card now sits.
func (s *Store) MoveTask(id, status string, index int) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("task status cannot be empty")
	}

	pos, err := s.positionAt(status, index, id)
	if err != nil {
		return err
	}

	res, err := s.db.Exec(
		`UPDATE tasks SET status = ?, position = ?, updated_at = ? WHERE id = ?`,
		status, pos, nowRFC3339Nano(), id,
	)
	if err != nil {
		return fmt.Errorf("move task: %w", err)
	}
	return requireOneRow(res, ErrTaskNotFound)
}

// LinkTaskSession attaches a session to a task, recording that work has begun.
func (s *Store) LinkTaskSession(taskID, sessionID string) error {
	res, err := s.db.Exec(
		`UPDATE tasks SET session_id = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(sessionID), nowRFC3339Nano(), strings.TrimSpace(taskID),
	)
	if err != nil {
		return fmt.Errorf("link task session: %w", err)
	}
	return requireOneRow(res, ErrTaskNotFound)
}

// DeleteTask removes a task. The linked session, if any, is left alone: the
// agent's work outlives the card that requested it.
func (s *Store) DeleteTask(id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return requireOneRow(res, ErrTaskNotFound)
}

// GetTaskByExternal finds the task mirroring a given upstream issue.
func (s *Store) GetTaskByExternal(provider, externalID string) (Task, error) {
	row := s.db.QueryRow(
		`SELECT `+taskColumns+` FROM tasks WHERE provider = ? AND external_id = ?`,
		strings.TrimSpace(provider), strings.TrimSpace(externalID),
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return t, err
}

// MarkTaskSynced records the upstream state observed during a sync pass.
func (s *Store) MarkTaskSynced(id, externalStatus string, at time.Time) error {
	res, err := s.db.Exec(
		`UPDATE tasks SET external_status = ?, synced_at = ? WHERE id = ?`,
		nullIfEmpty(externalStatus), at.UTC().Format(time.RFC3339Nano), strings.TrimSpace(id),
	)
	if err != nil {
		return fmt.Errorf("mark task synced: %w", err)
	}
	return requireOneRow(res, ErrTaskNotFound)
}

// ── positioning ──────────────────────────────────────────────────────────

// endPosition returns a position after every existing card in a column.
func (s *Store) endPosition(status string) (float64, error) {
	var max sql.NullFloat64
	err := s.db.QueryRow(`SELECT MAX(position) FROM tasks WHERE status = ?`, status).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("end position: %w", err)
	}
	if !max.Valid {
		return positionGap, nil
	}
	return max.Float64 + positionGap, nil
}

// positionAt computes the position for a card landing at index within a column,
// as the midpoint of its neighbours. excludeID keeps a card from being treated
// as its own neighbour when reordering inside the column it already occupies.
func (s *Store) positionAt(status string, index int, excludeID string) (float64, error) {
	rows, err := s.db.Query(
		`SELECT position FROM tasks WHERE status = ? AND id != ? ORDER BY position`,
		status, excludeID,
	)
	if err != nil {
		return 0, fmt.Errorf("column positions: %w", err)
	}
	defer rows.Close()

	positions := make([]float64, 0, 32)
	for rows.Next() {
		var p float64
		if err := rows.Scan(&p); err != nil {
			return 0, err
		}
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if index < 0 {
		index = 0
	}
	if index > len(positions) {
		index = len(positions)
	}

	switch {
	case len(positions) == 0:
		return positionGap, nil
	case index == 0:
		return positions[0] - positionGap, nil
	case index == len(positions):
		return positions[len(positions)-1] + positionGap, nil
	default:
		return (positions[index-1] + positions[index]) / 2, nil
	}
}

// ── scanning ─────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(sc rowScanner) (Task, error) {
	var (
		t                                          Task
		body, repoName, tool, model, baseBranch    sql.NullString
		sessionID, extID, extKey, extURL, extState sql.NullString
		syncedAt                                   sql.NullString
		createdRaw, updatedRaw                     string
	)

	err := sc.Scan(
		&t.ID, &t.Title, &body, &t.Status, &t.Position, &repoName, &tool, &model,
		&baseBranch, &sessionID, &t.Provider, &extID, &extKey, &extURL,
		&extState, &syncedAt, &createdRaw, &updatedRaw,
	)
	if err != nil {
		return Task{}, err
	}

	t.Body = body.String
	t.RepoName = repoName.String
	t.Tool = tool.String
	t.Model = model.String
	t.BaseBranch = baseBranch.String
	t.SessionID = sessionID.String
	t.ExternalID = extID.String
	t.ExternalKey = extKey.String
	t.ExternalURL = extURL.String
	t.ExternalStatus = extState.String

	if syncedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, syncedAt.String)
		if err != nil {
			return Task{}, fmt.Errorf("parse task synced_at: %w", err)
		}
		t.SyncedAt = &parsed
	}

	t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdRaw)
	if err != nil {
		return Task{}, fmt.Errorf("parse task created_at: %w", err)
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedRaw)
	if err != nil {
		return Task{}, fmt.Errorf("parse task updated_at: %w", err)
	}
	return t, nil
}

// ── small helpers ────────────────────────────────────────────────────────

// nullIfEmpty lives in sessions.go; reused here.

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func requireOneRow(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}
