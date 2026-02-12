package cloudrelay

import (
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/cloud"
	"github.com/darkLord19/foglet/internal/state"
)

func TestResolveBranchNameFromPromptAndPrefix(t *testing.T) {
	store, err := state.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new state store failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.SetSetting("branch_prefix", "team"); err != nil {
		t.Fatalf("set branch prefix failed: %v", err)
	}

	r := &Relay{stateStore: store}
	branch, err := r.resolveBranchName("", "Add OTP login using Redis")
	if err != nil {
		t.Fatalf("resolveBranchName failed: %v", err)
	}
	if !strings.HasPrefix(branch, "team/") {
		t.Fatalf("unexpected branch prefix: %q", branch)
	}
	if len(branch) > 255 {
		t.Fatalf("branch too long: %d", len(branch))
	}
}

func TestResolveBranchNameRequestedValidation(t *testing.T) {
	store, err := state.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new state store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	r := &Relay{stateStore: store}
	if _, err := r.resolveBranchName("feature/login", "prompt"); err != nil {
		t.Fatalf("unexpected error for valid requested branch: %v", err)
	}
	if _, err := r.resolveBranchName("bad branch name", "prompt"); err == nil {
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
