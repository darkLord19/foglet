package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/state"
)

func TestHandleReposList(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.stateStore.UpsertRepo(state.Repo{
		Name:             "acme/api",
		URL:              "https://github.com/acme/api.git",
		Host:             "github.com",
		Owner:            "acme",
		Repo:             "api",
		BarePath:         "/tmp/acme/api/repo.git",
		BaseWorktreePath: "/tmp/acme/api/base",
		DefaultBranch:    "main",
	})
	if err != nil {
		t.Fatalf("upsert repo failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	w := httptest.NewRecorder()
	srv.handleRepos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var repos []state.Repo
	if err := json.NewDecoder(w.Body).Decode(&repos); err != nil {
		t.Fatalf("decode repos failed: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "acme/api" {
		t.Fatalf("unexpected repos response: %+v", repos)
	}
}

func TestHandleDiscoverReposRequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	// Mock GH not authenticated
	origAuth := isGhAuthenticatedFn
	t.Cleanup(func() { isGhAuthenticatedFn = origAuth })
	isGhAuthenticatedFn = func() bool { return false }
	// Ensure GH is available
	origAvail := isGhAvailableFn
	t.Cleanup(func() { isGhAvailableFn = origAvail })
	isGhAvailableFn = func() bool { return true }

	req := httptest.NewRequest(http.MethodPost, "/api/repos/discover", nil)
	w := httptest.NewRecorder()

	srv.handleDiscoverRepos(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleDiscoverRepos(t *testing.T) {
	srv := newTestServer(t)

	// Mock GH authenticated
	origAuth := isGhAuthenticatedFn
	t.Cleanup(func() { isGhAuthenticatedFn = origAuth })
	isGhAuthenticatedFn = func() bool { return true }
	origAvail := isGhAvailableFn
	t.Cleanup(func() { isGhAvailableFn = origAvail })
	isGhAvailableFn = func() bool { return true }

	origDiscover := discoverReposFn
	t.Cleanup(func() { discoverReposFn = origDiscover })
	discoverReposFn = func() ([]ghcli.Repo, error) {
		return []ghcli.Repo{
			{Name: "api", NameWithOwner: "acme/api", URL: "https://github.com/acme/api", IsPrivate: true},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/repos/discover", nil)
	w := httptest.NewRecorder()
	srv.handleDiscoverRepos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var repos []ghcli.Repo
	if err := json.NewDecoder(w.Body).Decode(&repos); err != nil {
		t.Fatalf("decode repos failed: %v", err)
	}
	if len(repos) != 1 || repos[0].NameWithOwner != "acme/api" {
		t.Fatalf("unexpected discover response: %+v", repos)
	}
}

func TestHandleImportReposRequiresSelection(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/repos/import", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	srv.handleImportRepos(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleImportReposRequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	origAuth := isGhAuthenticatedFn
	origAvail := isGhAvailableFn
	t.Cleanup(func() {
		isGhAuthenticatedFn = origAuth
		isGhAvailableFn = origAvail
	})
	isGhAvailableFn = func() bool { return true }
	isGhAuthenticatedFn = func() bool { return false }

	req := httptest.NewRequest(http.MethodPost, "/api/repos/import", bytes.NewBufferString(`{"repos":["acme/api"]}`))
	w := httptest.NewRecorder()
	srv.handleImportRepos(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

func TestHandleImportRepos(t *testing.T) {
	srv := newTestServer(t)

	// Mock GH
	origAuth := isGhAuthenticatedFn
	t.Cleanup(func() { isGhAuthenticatedFn = origAuth })
	isGhAuthenticatedFn = func() bool { return true }
	origAvail := isGhAvailableFn
	t.Cleanup(func() { isGhAvailableFn = origAvail })
	isGhAvailableFn = func() bool { return true }

	origDiscover := discoverReposFn
	origImport := importReposFn
	t.Cleanup(func() {
		discoverReposFn = origDiscover
		importReposFn = origImport
	})

	discoverReposFn = func() ([]ghcli.Repo, error) {
		return []ghcli.Repo{
			{Name: "api", NameWithOwner: "acme/api", URL: "https://github.com/acme/api"},
		}, nil
	}

	importReposFn = func(fogHome string, store *state.Store, repos []ghcli.Repo) ([]string, error) {
		if len(repos) != 1 || repos[0].NameWithOwner != "acme/api" {
			t.Fatalf("unexpected import repos input: %+v", repos)
		}
		return []string{"acme/api"}, nil
	}

	body := bytes.NewBufferString(`{"repos":["acme/api"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/repos/import", body)
	w := httptest.NewRecorder()

	srv.handleImportRepos(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp importReposResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode import response failed: %v", err)
	}
	if !reflect.DeepEqual(resp.Imported, []string{"acme/api"}) {
		t.Fatalf("unexpected import response: %+v", resp)
	}
}

func TestSplitRepoFullNameValidation(t *testing.T) {
	owner, name, err := splitRepoFullName("acme/api")
	if err != nil {
		t.Fatalf("split repo full name failed: %v", err)
	}
	if owner != "acme" || name != "api" {
		t.Fatalf("unexpected split result: %s/%s", owner, name)
	}

	if _, _, err := splitRepoFullName("bad-format"); err == nil {
		t.Fatal("expected split failure for bad format")
	}
	if _, _, err := splitRepoFullName("../repo"); err == nil {
		t.Fatal("expected split failure for invalid segment")
	}
	if _, _, err := splitRepoFullName("owner/.."); err == nil {
		t.Fatal("expected split failure for invalid segment")
	}
	if _, _, err := splitRepoFullName("../evil/repo"); err == nil {
		t.Fatal("expected split failure for invalid segment")
	}
}

func TestRegisterRoutesListBranchesRoute(t *testing.T) {
	srv := newTestServer(t)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/repos/branches?name=missing/repo", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "unknown repo: missing/repo") {
		t.Fatalf("expected branch handler response, got body=%q", w.Body.String())
	}
}

func TestHandleListBranches(t *testing.T) {
	srv := newTestServer(t)
	repoPath, defaultBranch := initTestGitRepoWithFeatureBranch(t)

	_, err := srv.stateStore.UpsertRepo(state.Repo{
		Name:             "acme/api",
		URL:              "https://github.com/acme/api.git",
		Host:             "github.com",
		Owner:            "acme",
		Repo:             "api",
		BarePath:         filepath.Join(t.TempDir(), "repo.git"),
		BaseWorktreePath: repoPath,
		DefaultBranch:    defaultBranch,
	})
	if err != nil {
		t.Fatalf("upsert repo failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/branches?name=acme/api", nil)
	w := httptest.NewRecorder()
	srv.handleListBranches(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	type branchView struct {
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	}
	var branches []branchView
	if err := json.NewDecoder(w.Body).Decode(&branches); err != nil {
		t.Fatalf("decode branches failed: %v", err)
	}
	if len(branches) < 2 {
		t.Fatalf("expected at least 2 branches, got %+v", branches)
	}

	byName := make(map[string]branchView, len(branches))
	for _, b := range branches {
		byName[b.Name] = b
	}

	defaultEntry, ok := byName[defaultBranch]
	if !ok {
		t.Fatalf("default branch %q missing from response: %+v", defaultBranch, branches)
	}
	if !defaultEntry.IsDefault {
		t.Fatalf("default branch %q not marked default: %+v", defaultBranch, defaultEntry)
	}

	featureEntry, ok := byName["feature/test"]
	if !ok {
		t.Fatalf("feature branch missing from response: %+v", branches)
	}
	if featureEntry.IsDefault {
		t.Fatalf("feature branch should not be default: %+v", featureEntry)
	}
}

func TestHandleListBranchesMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/repos/branches?name=acme/api", nil)
	w := httptest.NewRecorder()

	srv.handleListBranches(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got %d want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func initTestGitRepoWithFeatureBranch(t *testing.T) (string, string) {
	t.Helper()

	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.email", "test@example.com")
	runGit(t, repoPath, "config", "user.name", "Fog Test")

	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write readme failed: %v", err)
	}
	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-m", "initial commit")

	defaultBranch := runGit(t, repoPath, "branch", "--show-current")
	if defaultBranch == "" {
		t.Fatal("default branch is empty")
	}

	runGit(t, repoPath, "checkout", "-b", "feature/test")
	runGit(t, repoPath, "checkout", defaultBranch)

	return repoPath, defaultBranch
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}
