package state

import (
	"errors"
	"testing"
	"time"
)

// One scanner per table means a column added to sessionColumns or runColumns
// reaches every query at once. These tests pin the round trip through both
// the single-row and multi-row paths, which used to be separate code.

func seedSessionAndRun(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.UpsertRepo(Repo{
		Name: "acme/api", URL: "https://github.com/acme/api.git",
		Host: "github.com", Owner: "acme", Repo: "api",
		BarePath: "/tmp/acme/repo.git", BaseWorktreePath: "/tmp/acme/base",
		DefaultBranch: "main",
	}); err != nil {
		t.Fatalf("upsert repo: %v", err)
	}

	now := time.Now().UTC()
	if err := s.CreateSession(Session{
		ID: "session-1", RepoName: "acme/api", Branch: "fog/test",
		WorktreePath: "/tmp/acme/wt", Tool: "claude", Model: "sonnet",
		AutoPR: true, PRURL: "https://example.invalid/pr/1",
		Status: "CREATED", Busy: true,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := s.CreateRun(Run{
		ID: "run-1", SessionID: "session-1", Prompt: "do work",
		WorktreePath: "/tmp/acme/wt-run-1", State: "CREATED",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
}

func TestGetSessionAndListSessionsAgree(t *testing.T) {
	s := newTestStore(t)
	seedSessionAndRun(t, s)

	single, found, err := s.GetSession("session-1")
	if err != nil || !found {
		t.Fatalf("GetSession: %v (found=%v)", err, found)
	}

	all, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ListSessions returned %d sessions, want 1", len(all))
	}

	if single.ID != all[0].ID || single.AutoPR != all[0].AutoPR ||
		single.Busy != all[0].Busy || single.PRURL != all[0].PRURL ||
		single.Tool != all[0].Tool || single.Model != all[0].Model ||
		!single.CreatedAt.Equal(all[0].CreatedAt) {
		t.Errorf("single-row and multi-row scans disagree:\n got %+v\nwant %+v", all[0], single)
	}
}

func TestSessionBooleanColumnsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	seedSessionAndRun(t, s)

	session, _, err := s.GetSession("session-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if !session.AutoPR {
		t.Error("AutoPR did not survive the round trip")
	}
	if !session.Busy {
		t.Error("Busy did not survive the round trip")
	}

	if err := s.SetSessionBusy("session-1", false); err != nil {
		t.Fatalf("SetSessionBusy: %v", err)
	}
	session, _, err = s.GetSession("session-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session.Busy {
		t.Error("Busy was not cleared")
	}
}

func TestGetRunListRunsAndGetLatestRunAgree(t *testing.T) {
	s := newTestStore(t)
	seedSessionAndRun(t, s)

	single, found, err := s.GetRun("run-1")
	if err != nil || !found {
		t.Fatalf("GetRun: %v (found=%v)", err, found)
	}

	listed, err := s.ListRuns("session-1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("ListRuns returned %d runs, want 1", len(listed))
	}

	latest, found, err := s.GetLatestRun("session-1")
	if err != nil || !found {
		t.Fatalf("GetLatestRun: %v (found=%v)", err, found)
	}

	for name, got := range map[string]Run{"ListRuns": listed[0], "GetLatestRun": latest} {
		if got.ID != single.ID || got.Prompt != single.Prompt ||
			got.WorktreePath != single.WorktreePath || got.State != single.State ||
			!got.CreatedAt.Equal(single.CreatedAt) {
			t.Errorf("%s disagrees with GetRun:\n got %+v\nwant %+v", name, got, single)
		}
	}
}

func TestCompletedAtIsNilUntilRunCompletes(t *testing.T) {
	s := newTestStore(t)
	seedSessionAndRun(t, s)

	run, _, err := s.GetRun("run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil for an incomplete run", run.CompletedAt)
	}

	if err := s.CompleteRun("run-1", "COMPLETED", "abc123", "feat: x", ""); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}

	run, _, err = s.GetRun("run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.CompletedAt == nil {
		t.Fatal("CompletedAt is nil after completion")
	}
	if run.CommitSHA != "abc123" || run.CommitMsg != "feat: x" {
		t.Errorf("commit fields did not round trip: %+v", run)
	}

	// The multi-row path must parse the nullable column identically.
	listed, err := s.ListRuns("session-1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if listed[0].CompletedAt == nil {
		t.Error("CompletedAt is nil via ListRuns but set via GetRun")
	}
}

// ── not-found convention ────────────────────────────────────────────────────

func TestMissingRowsReportErrNotFound(t *testing.T) {
	s := newTestStore(t)
	seedSessionAndRun(t, s)

	// Mutations addressed at a specific id wrap ErrNotFound so callers can
	// errors.Is instead of matching message text.
	for name, err := range map[string]error{
		"UpdateSessionStatus":    s.UpdateSessionStatus("ghost", "COMPLETED"),
		"SetSessionBusy":         s.SetSessionBusy("ghost", true),
		"SetSessionPRURL":        s.SetSessionPRURL("ghost", "https://example.invalid"),
		"SetSessionWorktreePath": s.SetSessionWorktreePath("ghost", "/tmp/x"),
		"SetRunState":            s.SetRunState("ghost", "AI_RUNNING"),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: error %v does not match ErrNotFound", name, err)
		}
	}
}

func TestErrTaskNotFoundMatchesErrNotFound(t *testing.T) {
	// Task callers use the specific sentinel; generic callers use ErrNotFound.
	// Both must match, or one of them silently stops working.
	if !errors.Is(ErrTaskNotFound, ErrNotFound) {
		t.Error("ErrTaskNotFound does not wrap ErrNotFound")
	}

	s := newTestStore(t)
	err := s.DeleteTask("ghost")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("DeleteTask: %v does not match ErrTaskNotFound", err)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteTask: %v does not match ErrNotFound", err)
	}
}

func TestLookupsReportAbsenceWithoutAnError(t *testing.T) {
	s := newTestStore(t)

	// Lookups that may legitimately miss use the found flag, not the sentinel.
	if _, found, err := s.GetSession("ghost"); err != nil || found {
		t.Errorf("GetSession(ghost) = (found=%v, err=%v), want (false, nil)", found, err)
	}
	if _, found, err := s.GetRun("ghost"); err != nil || found {
		t.Errorf("GetRun(ghost) = (found=%v, err=%v), want (false, nil)", found, err)
	}
	if _, found, err := s.GetLatestRun("ghost"); err != nil || found {
		t.Errorf("GetLatestRun(ghost) = (found=%v, err=%v), want (false, nil)", found, err)
	}
}
