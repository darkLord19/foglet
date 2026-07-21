package cloudrelay

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/cloud"
	"github.com/darkLord19/foglet/internal/state"
)

// The naming rules themselves are covered in internal/branchname. These tests
// cover only what the relay contributes: reading the configured prefix, and
// supplying the repository the uniqueness check runs against.

func newRelayTestStore(t *testing.T) *state.Store {
	t.Helper()
	store, err := state.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new state store failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func initRelayTestRepo(t *testing.T, branches ...string) string {
	t.Helper()
	repoPath := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "init")
	for _, branch := range branches {
		run("branch", branch)
	}
	return repoPath
}

func TestResolveBranchNameUsesConfiguredPrefix(t *testing.T) {
	store := newRelayTestStore(t)
	if err := store.SetSetting("branch_prefix", "team"); err != nil {
		t.Fatalf("set branch prefix failed: %v", err)
	}

	r := &Relay{stateStore: store}
	branch, err := r.resolveBranchName("", "", "Add OTP login using Redis")
	if err != nil {
		t.Fatalf("resolveBranchName failed: %v", err)
	}
	if branch != "team/add-otp-login-using-redis" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

// Regression: the relay used to omit the uniqueness check the runner performs, so
// a relayed job could be handed a branch name that already existed.
func TestResolveBranchNameAvoidsExistingBranches(t *testing.T) {
	repoPath := initRelayTestRepo(t, "fog/add-otp-login")

	r := &Relay{stateStore: newRelayTestStore(t)}
	branch, err := r.resolveBranchName(repoPath, "", "Add OTP login")
	if err != nil {
		t.Fatalf("resolveBranchName failed: %v", err)
	}
	if branch != "fog/add-otp-login-1" {
		t.Fatalf("expected a suffixed branch, got %q", branch)
	}
}

func TestResolveBranchNameWithoutRepoPathStillResolves(t *testing.T) {
	r := &Relay{stateStore: newRelayTestStore(t)}
	branch, err := r.resolveBranchName("", "", "Add OTP login")
	if err != nil {
		t.Fatalf("resolveBranchName failed: %v", err)
	}
	if branch != "fog/add-otp-login" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestResolveBranchNameRejectsInvalidRequestedName(t *testing.T) {
	r := &Relay{stateStore: newRelayTestStore(t)}
	if _, err := r.resolveBranchName("", "feature/login", "prompt"); err != nil {
		t.Fatalf("unexpected error for valid requested branch: %v", err)
	}
	if _, err := r.resolveBranchName("", "bad branch name", "prompt"); err == nil {
		t.Fatal("expected invalid branch name error")
	}
}

func TestHandleUnknownJobKind(t *testing.T) {
	r := &Relay{}
	out := r.handleJob(cloud.Job{Kind: "unknown"})
	if out.Success {
		t.Fatal("expected unknown kind to fail")
	}
	if !strings.Contains(out.Error, "unknown job kind") {
		t.Fatalf("unexpected error: %q", out.Error)
	}
}
