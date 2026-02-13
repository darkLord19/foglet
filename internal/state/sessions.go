package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Session represents one long-lived branch/worktree conversation.
type Session struct {
	ID           string    `json:"id"`
	RepoName     string    `json:"repo_name"`
	Branch       string    `json:"branch"`
	WorktreePath string    `json:"worktree_path"`
	Tool         string    `json:"tool"`
	Model        string    `json:"model,omitempty"`
	AutoPR       bool      `json:"autopr"`
	PRURL        string    `json:"pr_url,omitempty"`
	Status       string    `json:"status"`
	Busy         bool      `json:"busy"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Run is one execution step inside a session.
type Run struct {
	ID           string     `json:"id"`
	SessionID    string     `json:"session_id"`
	Prompt       string     `json:"prompt"`
	WorktreePath string     `json:"worktree_path"`
	State        string     `json:"state"`
	CommitSHA    string     `json:"commit_sha,omitempty"`
	CommitMsg    string     `json:"commit_msg,omitempty"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// RunEvent captures one timeline event for a run.
type RunEvent struct {
	ID      int64     `json:"id"`
	RunID   string    `json:"run_id"`
	TS      time.Time `json:"ts"`
	Type    string    `json:"type"`
	Message string    `json:"message,omitempty"`
	Data    string    `json:"data,omitempty"`
}

// CreateSession inserts a new session row.
func (s *Store) CreateSession(session Session) error {
	session.ID = strings.TrimSpace(session.ID)
	session.RepoName = strings.TrimSpace(session.RepoName)
	session.Branch = strings.TrimSpace(session.Branch)
	session.WorktreePath = strings.TrimSpace(session.WorktreePath)
	session.Tool = strings.TrimSpace(session.Tool)
	session.Status = strings.TrimSpace(session.Status)
	session.Model = strings.TrimSpace(session.Model)

	switch {
	case session.ID == "":
		return errors.New("session id cannot be empty")
	case session.RepoName == "":
		return errors.New("session repo_name cannot be empty")
	case session.Branch == "":
		return errors.New("session branch cannot be empty")
	case session.WorktreePath == "":
		return errors.New("session worktree_path cannot be empty")
	case session.Tool == "":
		return errors.New("session tool cannot be empty")
	case session.Status == "":
		return errors.New("session status cannot be empty")
	}

	createdAt := session.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := session.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	_, err := s.db.Exec(
		`INSERT INTO sessions(id, repo_name, branch, worktree_path, tool, model, autopr, pr_url, status, busy, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.RepoName,
		session.Branch,
		session.WorktreePath,
		session.Tool,
		session.Model,
		boolToInt(session.AutoPR),
		strings.TrimSpace(session.PRURL),
		session.Status,
		boolToInt(session.Busy),
		createdAt.Format(time.RFC3339Nano),
		updatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create session %q: %w", session.ID, err)
	}
	return nil
}

// GetSession returns one session by ID.
func (s *Store) GetSession(id string) (Session, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Session{}, false, errors.New("session id cannot be empty")
	}

	var session Session
	var autoPR int
	var busy int
	var createdAtRaw string
	var updatedAtRaw string
	err := s.db.QueryRow(
		`SELECT id, repo_name, branch, worktree_path, tool, model, autopr, pr_url, status, busy, created_at, updated_at
		   FROM sessions
		  WHERE id = ?`,
		id,
	).Scan(
		&session.ID,
		&session.RepoName,
		&session.Branch,
		&session.WorktreePath,
		&session.Tool,
		&session.Model,
		&autoPR,
		&session.PRURL,
		&session.Status,
		&busy,
		&createdAtRaw,
		&updatedAtRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("get session %q: %w", id, err)
	}

	session.AutoPR = autoPR == 1
	session.Busy = busy == 1
	session.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return Session{}, false, fmt.Errorf("parse session created_at %q: %w", id, err)
	}
	session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw)
	if err != nil {
		return Session{}, false, fmt.Errorf("parse session updated_at %q: %w", id, err)
	}

	return session, true, nil
}

// ListSessions returns all sessions sorted by most recently updated first.
func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT id, repo_name, branch, worktree_path, tool, model, autopr, pr_url, status, busy, created_at, updated_at
		   FROM sessions
		  ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]Session, 0)
	for rows.Next() {
		var session Session
		var autoPR int
		var busy int
		var createdAtRaw string
		var updatedAtRaw string
		if err := rows.Scan(
			&session.ID,
			&session.RepoName,
			&session.Branch,
			&session.WorktreePath,
			&session.Tool,
			&session.Model,
			&autoPR,
			&session.PRURL,
			&session.Status,
			&busy,
			&createdAtRaw,
			&updatedAtRaw,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		session.AutoPR = autoPR == 1
		session.Busy = busy == 1
		session.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse session created_at %q: %w", session.ID, err)
		}
		session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse session updated_at %q: %w", session.ID, err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, nil
}

// SetSessionBusy toggles the busy flag for a session.
func (s *Store) SetSessionBusy(id string, busy bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("session id cannot be empty")
	}

	res, err := s.db.Exec(
		`UPDATE sessions
		    SET busy = ?, updated_at = ?
		  WHERE id = ?`,
		boolToInt(busy),
		nowRFC3339Nano(),
		id,
	)
	if err != nil {
		return fmt.Errorf("set session busy %q: %w", id, err)
	}
	if err := ensureRowsAffected(res, "session "+id); err != nil {
		return err
	}
	return nil
}

// UpdateSessionStatus updates the session status value.
func (s *Store) UpdateSessionStatus(id, status string) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if id == "" {
		return errors.New("session id cannot be empty")
	}
	if status == "" {
		return errors.New("session status cannot be empty")
	}

	res, err := s.db.Exec(
		`UPDATE sessions
		    SET status = ?, updated_at = ?
		  WHERE id = ?`,
		status,
		nowRFC3339Nano(),
		id,
	)
	if err != nil {
		return fmt.Errorf("update session status %q: %w", id, err)
	}
	if err := ensureRowsAffected(res, "session "+id); err != nil {
		return err
	}
	return nil
}

// SetSessionPRURL stores a session-level PR URL.
func (s *Store) SetSessionPRURL(id, prURL string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("session id cannot be empty")
	}

	res, err := s.db.Exec(
		`UPDATE sessions
		    SET pr_url = ?, updated_at = ?
		  WHERE id = ?`,
		strings.TrimSpace(prURL),
		nowRFC3339Nano(),
		id,
	)
	if err != nil {
		return fmt.Errorf("set session pr_url %q: %w", id, err)
	}
	if err := ensureRowsAffected(res, "session "+id); err != nil {
		return err
	}
	return nil
}

// SetSessionWorktreePath updates the session's latest run worktree path.
func (s *Store) SetSessionWorktreePath(id, worktreePath string) error {
	id = strings.TrimSpace(id)
	worktreePath = strings.TrimSpace(worktreePath)
	if id == "" {
		return errors.New("session id cannot be empty")
	}
	if worktreePath == "" {
		return errors.New("session worktree_path cannot be empty")
	}

	res, err := s.db.Exec(
		`UPDATE sessions
		    SET worktree_path = ?, updated_at = ?
		  WHERE id = ?`,
		worktreePath,
		nowRFC3339Nano(),
		id,
	)
	if err != nil {
		return fmt.Errorf("set session worktree_path %q: %w", id, err)
	}
	if err := ensureRowsAffected(res, "session "+id); err != nil {
		return err
	}
	return nil
}

// CreateRun inserts a run under one session.
func (s *Store) CreateRun(run Run) error {
	run.ID = strings.TrimSpace(run.ID)
	run.SessionID = strings.TrimSpace(run.SessionID)
	run.Prompt = strings.TrimSpace(run.Prompt)
	run.WorktreePath = strings.TrimSpace(run.WorktreePath)
	run.State = strings.TrimSpace(run.State)
	run.CommitSHA = strings.TrimSpace(run.CommitSHA)
	run.CommitMsg = strings.TrimSpace(run.CommitMsg)
	run.Error = strings.TrimSpace(run.Error)

	switch {
	case run.ID == "":
		return errors.New("run id cannot be empty")
	case run.SessionID == "":
		return errors.New("run session_id cannot be empty")
	case run.Prompt == "":
		return errors.New("run prompt cannot be empty")
	case run.WorktreePath == "":
		return errors.New("run worktree_path cannot be empty")
	case run.State == "":
		return errors.New("run state cannot be empty")
	}

	createdAt := run.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := run.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	completedAtRaw := ""
	if run.CompletedAt != nil {
		completedAtRaw = run.CompletedAt.UTC().Format(time.RFC3339Nano)
	}

	_, err := s.db.Exec(
		`INSERT INTO runs(id, session_id, prompt, worktree_path, state, commit_sha, commit_msg, error, created_at, updated_at, completed_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID,
		run.SessionID,
		run.Prompt,
		run.WorktreePath,
		run.State,
		run.CommitSHA,
		run.CommitMsg,
		run.Error,
		createdAt.Format(time.RFC3339Nano),
		updatedAt.Format(time.RFC3339Nano),
		nullIfEmpty(completedAtRaw),
	)
	if err != nil {
		return fmt.Errorf("create run %q: %w", run.ID, err)
	}
	return nil
}

// GetRun returns one run by ID.
func (s *Store) GetRun(id string) (Run, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Run{}, false, errors.New("run id cannot be empty")
	}

	var run Run
	var createdAtRaw string
	var updatedAtRaw string
	var completedAtRaw sql.NullString
	err := s.db.QueryRow(
		`SELECT id, session_id, prompt, worktree_path, state, commit_sha, commit_msg, error, created_at, updated_at, completed_at
		   FROM runs
		  WHERE id = ?`,
		id,
	).Scan(
		&run.ID,
		&run.SessionID,
		&run.Prompt,
		&run.WorktreePath,
		&run.State,
		&run.CommitSHA,
		&run.CommitMsg,
		&run.Error,
		&createdAtRaw,
		&updatedAtRaw,
		&completedAtRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, false, nil
	}
	if err != nil {
		return Run{}, false, fmt.Errorf("get run %q: %w", id, err)
	}

	run.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return Run{}, false, fmt.Errorf("parse run created_at %q: %w", id, err)
	}
	run.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw)
	if err != nil {
		return Run{}, false, fmt.Errorf("parse run updated_at %q: %w", id, err)
	}
	if completedAtRaw.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, completedAtRaw.String)
		if err != nil {
			return Run{}, false, fmt.Errorf("parse run completed_at %q: %w", id, err)
		}
		run.CompletedAt = &parsed
	}

	return run, true, nil
}

// ListRuns returns runs in one session, newest first.
func (s *Store) ListRuns(sessionID string) ([]Run, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session id cannot be empty")
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, prompt, worktree_path, state, commit_sha, commit_msg, error, created_at, updated_at, completed_at
		   FROM runs
		  WHERE session_id = ?
		  ORDER BY created_at DESC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list runs for session %q: %w", sessionID, err)
	}
	defer rows.Close()

	runs := make([]Run, 0)
	for rows.Next() {
		var run Run
		var createdAtRaw string
		var updatedAtRaw string
		var completedAtRaw sql.NullString
		if err := rows.Scan(
			&run.ID,
			&run.SessionID,
			&run.Prompt,
			&run.WorktreePath,
			&run.State,
			&run.CommitSHA,
			&run.CommitMsg,
			&run.Error,
			&createdAtRaw,
			&updatedAtRaw,
			&completedAtRaw,
		); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		run.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse run created_at %q: %w", run.ID, err)
		}
		run.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse run updated_at %q: %w", run.ID, err)
		}
		if completedAtRaw.Valid {
			parsed, err := time.Parse(time.RFC3339Nano, completedAtRaw.String)
			if err != nil {
				return nil, fmt.Errorf("parse run completed_at %q: %w", run.ID, err)
			}
			run.CompletedAt = &parsed
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}

// GetLatestRun returns the most recently created run for a session.
func (s *Store) GetLatestRun(sessionID string) (Run, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Run{}, false, errors.New("session id cannot be empty")
	}

	var run Run
	var createdAtRaw string
	var updatedAtRaw string
	var completedAtRaw sql.NullString
	err := s.db.QueryRow(
		`SELECT id, session_id, prompt, worktree_path, state, commit_sha, commit_msg, error, created_at, updated_at, completed_at
		   FROM runs
		  WHERE session_id = ?
		  ORDER BY created_at DESC
		  LIMIT 1`,
		sessionID,
	).Scan(
		&run.ID,
		&run.SessionID,
		&run.Prompt,
		&run.WorktreePath,
		&run.State,
		&run.CommitSHA,
		&run.CommitMsg,
		&run.Error,
		&createdAtRaw,
		&updatedAtRaw,
		&completedAtRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, false, nil
	}
	if err != nil {
		return Run{}, false, fmt.Errorf("get latest run for session %q: %w", sessionID, err)
	}

	run.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return Run{}, false, fmt.Errorf("parse run created_at %q: %w", run.ID, err)
	}
	run.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw)
	if err != nil {
		return Run{}, false, fmt.Errorf("parse run updated_at %q: %w", run.ID, err)
	}
	if completedAtRaw.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, completedAtRaw.String)
		if err != nil {
			return Run{}, false, fmt.Errorf("parse run completed_at %q: %w", run.ID, err)
		}
		run.CompletedAt = &parsed
	}
	return run, true, nil
}

// SetRunState updates only the state and updated timestamp.
func (s *Store) SetRunState(id, state string) error {
	id = strings.TrimSpace(id)
	state = strings.TrimSpace(state)
	if id == "" {
		return errors.New("run id cannot be empty")
	}
	if state == "" {
		return errors.New("run state cannot be empty")
	}

	res, err := s.db.Exec(
		`UPDATE runs
		    SET state = ?, updated_at = ?
		  WHERE id = ?`,
		state,
		nowRFC3339Nano(),
		id,
	)
	if err != nil {
		return fmt.Errorf("set run state %q: %w", id, err)
	}
	if err := ensureRowsAffected(res, "run "+id); err != nil {
		return err
	}
	return nil
}

// CompleteRun stores terminal run data and sets completed_at.
func (s *Store) CompleteRun(id, state, commitSHA, commitMsg, runErr string) error {
	id = strings.TrimSpace(id)
	state = strings.TrimSpace(state)
	if id == "" {
		return errors.New("run id cannot be empty")
	}
	if state == "" {
		return errors.New("run state cannot be empty")
	}

	now := nowRFC3339Nano()
	res, err := s.db.Exec(
		`UPDATE runs
		    SET state = ?, commit_sha = ?, commit_msg = ?, error = ?, updated_at = ?, completed_at = ?
		  WHERE id = ?`,
		state,
		strings.TrimSpace(commitSHA),
		strings.TrimSpace(commitMsg),
		strings.TrimSpace(runErr),
		now,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("complete run %q: %w", id, err)
	}
	if err := ensureRowsAffected(res, "run "+id); err != nil {
		return err
	}
	return nil
}

// AppendRunEvent inserts one run event entry.
func (s *Store) AppendRunEvent(event RunEvent) error {
	event.RunID = strings.TrimSpace(event.RunID)
	event.Type = strings.TrimSpace(event.Type)
	event.Message = strings.TrimSpace(event.Message)
	event.Data = strings.TrimSpace(event.Data)
	if event.RunID == "" {
		return errors.New("run event run_id cannot be empty")
	}
	if event.Type == "" {
		return errors.New("run event type cannot be empty")
	}

	ts := event.TS.UTC()
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO run_events(run_id, ts, type, message, data)
		 VALUES(?, ?, ?, ?, ?)`,
		event.RunID,
		ts.Format(time.RFC3339Nano),
		event.Type,
		nullIfEmpty(event.Message),
		nullIfEmpty(event.Data),
	)
	if err != nil {
		return fmt.Errorf("append run event for %q: %w", event.RunID, err)
	}
	return nil
}

// ListRunEvents returns run events in chronological order.
func (s *Store) ListRunEvents(runID string, limit int) ([]RunEvent, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, errors.New("run id cannot be empty")
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows, err := s.db.Query(
		`SELECT id, run_id, ts, type, message, data
		   FROM run_events
		  WHERE run_id = ?
		  ORDER BY id ASC
		  LIMIT ?`,
		runID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list run events for %q: %w", runID, err)
	}
	defer rows.Close()

	events := make([]RunEvent, 0)
	for rows.Next() {
		var event RunEvent
		var tsRaw string
		var message sql.NullString
		var data sql.NullString
		if err := rows.Scan(
			&event.ID,
			&event.RunID,
			&tsRaw,
			&event.Type,
			&message,
			&data,
		); err != nil {
			return nil, fmt.Errorf("scan run event: %w", err)
		}
		event.TS, err = time.Parse(time.RFC3339Nano, tsRaw)
		if err != nil {
			return nil, fmt.Errorf("parse run event ts for %q: %w", runID, err)
		}
		event.Message = message.String
		event.Data = data.String
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run events: %w", err)
	}
	return events, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func ensureRowsAffected(res sql.Result, objectName string) error {
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for %s: %w", objectName, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s not found", objectName)
	}
	return nil
}
