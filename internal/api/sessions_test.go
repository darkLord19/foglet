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

func TestHandleCreateFollowUpRunRequiresPrompt(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/abc/runs", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	srv.handleSessionDetail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestResolveBranchNameUsesPrefixAndSlugifiesPrompt(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.stateStore.SetSetting("branch_prefix", "team"); err != nil {
		t.Fatalf("set branch prefix failed: %v", err)
	}

	branch, err := srv.resolveBranchName("", "Add OTP Login!!")
	if err != nil {
		t.Fatalf("resolveBranchName failed: %v", err)
	}
	if branch != "team/add-otp-login" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestValidateBranchNameRejectsInvalidSequences(t *testing.T) {
	_, err := validateBranchName("feature//bad")
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
		ID:        "run-1",
		SessionID: "session-1",
		Prompt:    "add otp login",
		State:     "CREATED",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create run failed: %v", err)
	}
}
