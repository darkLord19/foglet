package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
)

func TestHandleCreateTaskRequiresRepo(t *testing.T) {
	srv := newTestServer(t)

	body := []byte(`{"branch":"feature-a","prompt":"do thing"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleCreateTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTaskRejectsUnknownRepo(t *testing.T) {
	srv := newTestServer(t)

	body := []byte(`{"repo":"missing","branch":"feature-a","prompt":"do thing"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleCreateTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSettingsGet(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.stateStore.SetSetting("branch_prefix", "fog"); err != nil {
		t.Fatalf("set branch prefix failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()

	srv.handleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusOK)
	}

	var resp SettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.DefaultTool != "claude" {
		t.Fatalf("unexpected default tool: %s", resp.DefaultTool)
	}
	if resp.BranchPrefix != "fog" {
		t.Fatalf("unexpected branch prefix: %s", resp.BranchPrefix)
	}
}

func TestHandleSettingsPut(t *testing.T) {
	srv := newTestServer(t)
	body := bytes.NewBufferString(`{"default_tool":"claude","branch_prefix":"team"}`)

	req := httptest.NewRequest(http.MethodPut, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	prefix, found, err := srv.stateStore.GetSetting("branch_prefix")
	if err != nil {
		t.Fatalf("read branch prefix failed: %v", err)
	}
	if !found || prefix != "team" {
		t.Fatalf("unexpected branch prefix in state store: %q found=%v", prefix, found)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfgDir := t.TempDir()
	r, err := runner.New("", cfgDir)
	if err != nil {
		t.Fatalf("new runner failed: %v", err)
	}

	st, err := state.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new state store failed: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.SetDefaultTool("claude"); err != nil {
		t.Fatalf("set default tool failed: %v", err)
	}

	return New(r, st, 8080)
}
