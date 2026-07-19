package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/darkLord19/foglet/internal/state"
)

func TestHandleSessionsGet(t *testing.T) {
	srv := newTestServer(t)
	seedSessionFixture(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()

	srv.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var sessions []state.Session
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("unexpected session count: got %d want 1", len(sessions))
	}
	if sessions[0].RepoName != "acme/api" {
		t.Fatalf("unexpected repo name in response: %q", sessions[0].RepoName)
	}
}

func TestHandleCreateSessionRequiresRepoAndPrompt(t *testing.T) {
	srv := newTestServer(t)
	body := bytes.NewBufferString(`{"prompt":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	w := httptest.NewRecorder()

	srv.handleSessions(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSessionDetailNotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/missing", nil)
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleSessionCancelRoute(t *testing.T) {
	srv := newTestServer(t)
	seedSessionFixture(t, srv)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/session-1/cancel", nil)
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleSessionDiffRoute(t *testing.T) {
	srv := newTestServer(t)
	seedSessionFixture(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/diff", nil)
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleSessionOpenRoute(t *testing.T) {
	srv := newTestServer(t)
	seedSessionFixture(t, srv)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/session-1/open", nil)
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d or %d body=%s", w.Code, http.StatusOK, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleSessionStreamRoute(t *testing.T) {
	srv := newTestServer(t)
	seedSessionFixture(t, srv)
	if err := srv.stateStore.CompleteRun("run-1", "COMPLETED", "", "", ""); err != nil {
		t.Fatalf("complete run failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/runs/run-1/stream", nil)
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "event: done") {
		t.Fatalf("expected done event in stream output, got: %s", w.Body.String())
	}
}

func TestHandleCreateFollowUpRunRequiresPrompt(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/abc/runs", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleForkSessionRequiresPrompt(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/abc/fork", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleForkSessionUnknownSession(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/missing/fork", bytes.NewBufferString(`{"prompt":"new fork prompt"}`))
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestResolveBranchNameUsesPrefixAndSlugifiesPrompt(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.stateStore.SetSetting("branch_prefix", "team"); err != nil {
		t.Fatalf("set branch prefix failed: %v", err)
	}

	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.email", "test@example.com")
	runGit(t, repoPath, "config", "user.name", "Test User")
	runGit(t, repoPath, "commit", "--allow-empty", "-m", "init")

	branch, err := srv.runner.ResolveBranch(repoPath, "", "Add OTP Login!!")
	if err != nil {
		t.Fatalf("ResolveBranch failed: %v", err)
	}
	if branch != "team/add-otp-login" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestResolveBranchNameRejectsInvalidSequences(t *testing.T) {
	srv := newTestServer(t)

	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.email", "test@example.com")
	runGit(t, repoPath, "config", "user.name", "Test User")
	runGit(t, repoPath, "commit", "--allow-empty", "-m", "init")

	_, err := srv.runner.ResolveBranch(repoPath, "feature//bad", "")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func seedSessionFixture(t *testing.T, srv *Server) {
	t.Helper()
	_, err := srv.stateStore.UpsertRepo(state.Repo{
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

	now := time.Now().UTC()
	if err := srv.stateStore.CreateSession(state.Session{
		ID:           "session-1",
		RepoName:     "acme/api",
		Branch:       "team/add-otp-login",
		WorktreePath: "/tmp/acme-api/worktree",
		Tool:         "claude",
		Model:        "sonnet",
		AutoPR:       true,
		Status:       "CREATED",
		Busy:         false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if err := srv.stateStore.CreateRun(state.Run{
		ID:           "run-1",
		SessionID:    "session-1",
		Prompt:       "add otp login",
		WorktreePath: "/tmp/acme-api/worktree-run-1",
		State:        "CREATED",
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("create run failed: %v", err)
	}
}

func TestResolveBranchName_Unique(t *testing.T) {
	srv := newTestServer(t)

	// 1. Setup a dummy repo
	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.email", "test@example.com")
	runGit(t, repoPath, "config", "user.name", "Test User")
	runGit(t, repoPath, "commit", "--allow-empty", "-m", "init")

	// 2. Create a branch "fog/task-collision"
	branchName := "fog/task-collision"
	runGit(t, repoPath, "branch", branchName)

	// 3. Call ResolveBranch with a prompt that produces "task-collision" slug
	prompt := "Task Collision"

	uniqueName, err := srv.runner.ResolveBranch(repoPath, "", prompt)
	if err != nil {
		t.Fatalf("ResolveBranch failed: %v", err)
	}

	// 4. Assert uniqueName is NOT "fog/task-collision"
	if uniqueName == branchName {
		t.Errorf("Expected unique name, got collision: %s", uniqueName)
	}
	if !strings.HasPrefix(uniqueName, "fog/task-collision-") {
		t.Errorf("Expected suffix on collision, got: %s", uniqueName)
	}
	t.Logf("Resolved unique branch: %s", uniqueName)
}
