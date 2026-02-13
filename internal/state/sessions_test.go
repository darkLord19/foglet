package state

import (
	"strings"
	"testing"
	"time"
)

func TestSessionAndRunLifecycle(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	_, err := store.UpsertRepo(Repo{
		Name:             "acme/api",
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

	session := Session{
		ID:           "sess-1",
		RepoName:     "acme/api",
		Branch:       "fog/add-login",
		WorktreePath: "/tmp/acme-api/branches/fog-add-login",
		Tool:         "claude",
		Model:        "sonnet",
		AutoPR:       true,
		Status:       "CREATED",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	gotSession, found, err := store.GetSession("sess-1")
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}
	if !found {
		t.Fatal("expected session to exist")
	}
	if gotSession.RepoName != "acme/api" || gotSession.Tool != "claude" {
		t.Fatalf("unexpected session payload: %+v", gotSession)
	}

	if err := store.SetSessionBusy("sess-1", true); err != nil {
		t.Fatalf("set session busy failed: %v", err)
	}
	if err := store.UpdateSessionStatus("sess-1", "AI_RUNNING"); err != nil {
		t.Fatalf("update session status failed: %v", err)
	}
	if err := store.SetSessionPRURL("sess-1", "https://github.com/acme/api/pull/1"); err != nil {
		t.Fatalf("set session pr url failed: %v", err)
	}

	gotSession, found, err = store.GetSession("sess-1")
	if err != nil {
		t.Fatalf("get session after update failed: %v", err)
	}
	if !found {
		t.Fatal("expected session to exist after update")
	}
	if !gotSession.Busy || gotSession.Status != "AI_RUNNING" {
		t.Fatalf("unexpected session state after update: %+v", gotSession)
	}
	if !strings.Contains(gotSession.PRURL, "/pull/1") {
		t.Fatalf("expected PR URL to be stored, got %q", gotSession.PRURL)
	}

	r1Created := time.Now().Add(-1 * time.Minute).UTC()
	r2Created := time.Now().UTC()
	if err := store.CreateRun(Run{
		ID:           "run-1",
		SessionID:    "sess-1",
		Prompt:       "Add OTP login",
		WorktreePath: "/tmp/acme-api/branches/fog-add-login-run-1",
		State:        "CREATED",
		CreatedAt:    r1Created,
		UpdatedAt:    r1Created,
	}); err != nil {
		t.Fatalf("create run-1 failed: %v", err)
	}
	if err := store.CreateRun(Run{
		ID:           "run-2",
		SessionID:    "sess-1",
		Prompt:       "Add tests",
		WorktreePath: "/tmp/acme-api/branches/fog-add-login-run-2",
		State:        "CREATED",
		CreatedAt:    r2Created,
		UpdatedAt:    r2Created,
	}); err != nil {
		t.Fatalf("create run-2 failed: %v", err)
	}

	runs, err := store.ListRuns("sess-1")
	if err != nil {
		t.Fatalf("list runs failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("unexpected run count: got %d want 2", len(runs))
	}
	if runs[0].ID != "run-2" {
		t.Fatalf("expected newest run first, got %+v", runs)
	}
	if got, want := runs[0].WorktreePath, "/tmp/acme-api/branches/fog-add-login-run-2"; got != want {
		t.Fatalf("unexpected run worktree path: got %q want %q", got, want)
	}
	latest, found, err := store.GetLatestRun("sess-1")
	if err != nil {
		t.Fatalf("get latest run failed: %v", err)
	}
	if !found || latest.ID != "run-2" {
		t.Fatalf("expected run-2 to be latest, got %+v", latest)
	}

	if err := store.SetRunState("run-1", "AI_RUNNING"); err != nil {
		t.Fatalf("set run state failed: %v", err)
	}
	if err := store.CompleteRun("run-1", "COMPLETED", "abc1234", "feat: add otp login", ""); err != nil {
		t.Fatalf("complete run failed: %v", err)
	}

	gotRun, found, err := store.GetRun("run-1")
	if err != nil {
		t.Fatalf("get completed run failed: %v", err)
	}
	if !found {
		t.Fatal("expected run-1 to exist")
	}
	if gotRun.State != "COMPLETED" || gotRun.CommitSHA != "abc1234" {
		t.Fatalf("unexpected completed run payload: %+v", gotRun)
	}
	if gotRun.CompletedAt == nil {
		t.Fatalf("expected completed_at to be set: %+v", gotRun)
	}

	if err := store.SetSessionWorktreePath("sess-1", "/tmp/acme-api/branches/fog-add-login-run-2"); err != nil {
		t.Fatalf("set session worktree path failed: %v", err)
	}
	gotSession, found, err = store.GetSession("sess-1")
	if err != nil {
		t.Fatalf("get session after worktree update failed: %v", err)
	}
	if !found {
		t.Fatal("expected session to exist after worktree update")
	}
	if gotSession.WorktreePath != "/tmp/acme-api/branches/fog-add-login-run-2" {
		t.Fatalf("unexpected session worktree path: %q", gotSession.WorktreePath)
	}

	if err := store.AppendRunEvent(RunEvent{
		RunID:   "run-1",
		Type:    "STATE",
		Message: "AI started",
	}); err != nil {
		t.Fatalf("append run event 1 failed: %v", err)
	}
	if err := store.AppendRunEvent(RunEvent{
		RunID:   "run-1",
		Type:    "STATE",
		Message: "Commit created",
	}); err != nil {
		t.Fatalf("append run event 2 failed: %v", err)
	}

	events, err := store.ListRunEvents("run-1", 10)
	if err != nil {
		t.Fatalf("list run events failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("unexpected run event count: got %d want 2", len(events))
	}
	if events[0].Message != "AI started" {
		t.Fatalf("expected insertion order for events, got %+v", events)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("list sessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sess-1" {
		t.Fatalf("unexpected sessions payload: %+v", sessions)
	}
}

func TestCreateSessionRequiresExistingRepo(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	err := store.CreateSession(Session{
		ID:           "sess-missing-repo",
		RepoName:     "missing/repo",
		Branch:       "fog/x",
		WorktreePath: "/tmp/x",
		Tool:         "claude",
		Status:       "CREATED",
	})
	if err == nil {
		t.Fatal("expected fk error when repo does not exist")
	}
}

func TestSessionRunUpdateMissingRows(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SetSessionBusy("missing-session", true); err == nil {
		t.Fatal("expected missing session error")
	}
	if err := store.SetRunState("missing-run", "AI_RUNNING"); err == nil {
		t.Fatal("expected missing run error")
	}
	if err := store.CompleteRun("missing-run", "FAILED", "", "", "boom"); err == nil {
		t.Fatal("expected missing run error")
	}
}
