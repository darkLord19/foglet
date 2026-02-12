package main

import (
	"testing"

	fogenv "github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/state"
)

func TestValidateBranchPrefix(t *testing.T) {
	if err := validateBranchPrefix(" "); err == nil {
		t.Fatalf("expected empty prefix error")
	}
	if err := validateBranchPrefix("team"); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValueHelpers(t *testing.T) {
	if got := valueOrUnset(""); got != "(unset)" {
		t.Fatalf("valueOrUnset mismatch: %q", got)
	}
	if got := valueOrUnset("claude"); got != "claude" {
		t.Fatalf("valueOrUnset mismatch: %q", got)
	}
	if got := boolLabel(true); got != "configured" {
		t.Fatalf("boolLabel(true) mismatch: %q", got)
	}
	if got := boolLabel(false); got != "missing" {
		t.Fatalf("boolLabel(false) mismatch: %q", got)
	}
}

func TestLoadFogConfigView(t *testing.T) {
	fogHome := t.TempDir()
	store, err := state.NewStore(fogHome)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SetDefaultTool("claude"); err != nil {
		t.Fatalf("set default tool failed: %v", err)
	}
	if err := store.SetSetting("branch_prefix", "team"); err != nil {
		t.Fatalf("set branch prefix failed: %v", err)
	}

	view, err := loadFogConfigView(store, fogHome)
	if err != nil {
		t.Fatalf("loadFogConfigView failed: %v", err)
	}

	if view.DefaultTool != "claude" {
		t.Fatalf("unexpected default tool: %s", view.DefaultTool)
	}
	if view.BranchPrefix != "team" {
		t.Fatalf("unexpected branch prefix: %s", view.BranchPrefix)
	}
	if view.ManagedRepos != fogenv.ManagedReposDir(fogHome) {
		t.Fatalf("unexpected managed repos dir: %s", view.ManagedRepos)
	}
}
