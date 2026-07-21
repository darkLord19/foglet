package state

import (
	"database/sql"
	"fmt"
	"time"
)

// Column lists and row scanners for sessions and runs.
//
// These previously did not exist: the 11-column run scan-and-parse block was
// written out in full in GetRun, ListRuns and GetLatestRun, and the session
// scan twice more in GetSession and ListSessions. Five copies of the same
// column order is five chances for a schema change to be applied to four of
// them. internal/state/tasks.go already had this shape (taskColumns, scanTask);
// this brings sessions and runs into line.
//
// rowScanner is declared in tasks.go and satisfied by both *sql.Row and
// *sql.Rows, so one scanner serves the single-row and multi-row queries.

const sessionColumns = `id, repo_name, branch, worktree_path, tool, model,
	autopr, pr_url, status, busy, created_at, updated_at`

const runColumns = `id, session_id, prompt, worktree_path, state,
	commit_sha, commit_msg, error, created_at, updated_at, completed_at`

// scanSession reads one session row. The column order must match sessionColumns.
func scanSession(sc rowScanner) (Session, error) {
	var (
		session      Session
		autoPR, busy int
		createdAtRaw string
		updatedAtRaw string
	)
	if err := sc.Scan(
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
		return Session{}, err
	}

	session.AutoPR = autoPR == 1
	session.Busy = busy == 1

	var err error
	if session.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw); err != nil {
		return Session{}, fmt.Errorf("parse session created_at %q: %w", session.ID, err)
	}
	if session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw); err != nil {
		return Session{}, fmt.Errorf("parse session updated_at %q: %w", session.ID, err)
	}
	return session, nil
}

// scanRun reads one run row. The column order must match runColumns.
func scanRun(sc rowScanner) (Run, error) {
	var (
		run            Run
		createdAtRaw   string
		updatedAtRaw   string
		completedAtRaw sql.NullString
	)
	if err := sc.Scan(
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
		return Run{}, err
	}

	var err error
	if run.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw); err != nil {
		return Run{}, fmt.Errorf("parse run created_at %q: %w", run.ID, err)
	}
	if run.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw); err != nil {
		return Run{}, fmt.Errorf("parse run updated_at %q: %w", run.ID, err)
	}
	if completedAtRaw.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, completedAtRaw.String)
		if err != nil {
			return Run{}, fmt.Errorf("parse run completed_at %q: %w", run.ID, err)
		}
		run.CompletedAt = &parsed
	}
	return run, nil
}
