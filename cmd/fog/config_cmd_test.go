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
	if got := installedLabel(true); got != "installed" {
		t.Fatalf("installedLabel(true) mismatch: %q", got)
	}
	if got := installedLabel(false); got != "missing" {
		t.Fatalf("installedLabel(false) mismatch: %q", got)
	}
	if got := authenticatedLabel(true); got != "authenticated" {
		t.Fatalf("authenticatedLabel(true) mismatch: %q", got)
	}
	if got := authenticatedLabel(false); got != "missing" {
		t.Fatalf("authenticatedLabel(false) mismatch: %q", got)
	}
}

func TestLoadFogConfigView(t *testing.T) {
	origAvail := isGhAvailableFn
	origAuth := isGhAuthenticatedFn
	t.Cleanup(func() {
		isGhAvailableFn = origAvail
		isGhAuthenticatedFn = origAuth
	})
	isGhAvailableFn = func() bool { return true }
	isGhAuthenticatedFn = func() bool { return false }

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
	if !view.GhInstalled {
		t.Fatalf("expected gh to be marked installed")
	}
	if view.GhAuthenticated {
		t.Fatalf("expected gh to be marked not authenticated")
	}
}
