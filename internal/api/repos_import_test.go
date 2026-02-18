package api

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/state"
)

func TestImportSelectedRepos_Parallel(t *testing.T) {
	// Setup temp home
	tmpHome := t.TempDir()

	// Setup state store
	store, err := state.NewStore(tmpHome)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	// Mock git command
	origGit := runGitCommandFn
	defer func() { runGitCommandFn = origGit }()
	runGitCommandFn = func(args ...string) error {
		return nil // Success
	}

	// Mock ghcli clone
	origClone := ghcliCloneRepoFn
	defer func() { ghcliCloneRepoFn = origClone }()

	var cloneCount int32
	ghcliCloneRepoFn = func(fullName, destPath string) error {
		atomic.AddInt32(&cloneCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return nil
	}

	// Create inputs
	repos := make([]ghcli.Repo, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("acme/repo-%d", i)
		repos[i] = ghcli.Repo{
			Name:          fmt.Sprintf("repo-%d", i),
			NameWithOwner: name,
			URL:           fmt.Sprintf("https://github.com/%s", name),
			Owner: struct {
				Login string `json:"login"`
			}{Login: "acme"},
			DefaultBranchRef: struct {
				Name string `json:"name"`
			}{Name: "main"},
		}
	}

	// Run import
	start := time.Now()
	imported, err := importReposFn(tmpHome, store, repos)
	if err != nil {
		t.Fatalf("importReposFn failed: %v", err)
	}
	duration := time.Since(start)

	// Verify
	if len(imported) != 10 {
		t.Errorf("expected 10 imported, got %d", len(imported))
	}
	if atomic.LoadInt32(&cloneCount) != 10 {
		t.Errorf("expected 10 clones, got %d", cloneCount)
	}

	// With 10 repos and batch size 5, and 10ms sleep, it should take roughly 20ms+overhead,
	// definitely less than 100ms (sequential).
	// Just logging duration for sanity check, strict assertion on time is flaky.
	t.Logf("Import took %v", duration)
}

func TestImportSelectedRepos_Interrupted_InvalidRepo(t *testing.T) {
	tmpHome := t.TempDir()

	store, err := state.NewStore(tmpHome)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	repo := ghcli.Repo{
		Name:          "repo-invalid",
		NameWithOwner: "acme/repo-invalid",
		URL:           "https://github.com/acme/repo-invalid",
		Owner: struct {
			Login string `json:"login"`
		}{Login: "acme"},
		DefaultBranchRef: struct {
			Name string `json:"name"`
		}{Name: "main"},
	}

	// Prepare invalid repo directory "bare"
	managedDir := filepath.Join(tmpHome, "repos", "acme", "repo-invalid")
	barePath := filepath.Join(managedDir, "repo.git")
	if err := os.MkdirAll(barePath, 0755); err != nil {
		t.Fatalf("setup invalid repo dir: %v", err)
	}

	// Mock git command
	origGit := runGitCommandFn
	defer func() { runGitCommandFn = origGit }()
	runGitCommandFn = func(args ...string) error {
		// Mock rev-parse --git-dir failure to simulate invalid repo
		if len(args) > 0 && args[0] == "--git-dir" {
			// Check if validating the bare path
			if args[1] == barePath && args[2] == "rev-parse" {
				return fmt.Errorf("fatal: not a git repository")
			}
		}
		return nil
	}

	// Mock ghcli clone
	origClone := ghcliCloneRepoFn
	defer func() { ghcliCloneRepoFn = origClone }()

	var cloneCalled bool
	ghcliCloneRepoFn = func(fullName, destPath string) error {
		cloneCalled = true
		// Verify it's cloning to the right place
		if destPath != barePath {
			return fmt.Errorf("unexpected clone dest: %s", destPath)
		}
		// Creating the dir to simulate clone success
		return os.MkdirAll(destPath, 0755)
	}

	// Run import
	imported, err := importReposFn(tmpHome, store, []ghcli.Repo{repo})
	if err != nil {
		t.Fatalf("importReposFn failed: %v", err)
	}

	if len(imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(imported))
	}
	if !cloneCalled {
		t.Error("expected clone to be called for invalid repo, but it wasn't")
	}
}

func TestImportSelectedRepos_Interrupted_ValidRepo(t *testing.T) {
	tmpHome := t.TempDir()

	store, err := state.NewStore(tmpHome)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	repo := ghcli.Repo{
		Name:          "repo-valid",
		NameWithOwner: "acme/repo-valid",
		URL:           "https://github.com/acme/repo-valid",
		Owner: struct {
			Login string `json:"login"`
		}{Login: "acme"},
		DefaultBranchRef: struct {
			Name string `json:"name"`
		}{Name: "main"},
	}

	// Prepare VALID repo directory "bare"
	managedDir := filepath.Join(tmpHome, "repos", "acme", "repo-valid")
	barePath := filepath.Join(managedDir, "repo.git")
	if err := os.MkdirAll(barePath, 0755); err != nil {
		t.Fatalf("setup valid repo dir: %v", err)
	}

	// Mock git command
	origGit := runGitCommandFn
	defer func() { runGitCommandFn = origGit }()
	runGitCommandFn = func(args ...string) error {
		// Mock rev-parse --git-dir SUCCESS
		if len(args) > 0 && args[0] == "--git-dir" {
			if args[1] == barePath && args[2] == "rev-parse" {
				return nil
			}
			t.Logf("git command mismatch: got args %v, want [..., %s, rev-parse, ...]", args, barePath)
		}
		return nil
	}

	// Mock ghcli clone
	origClone := ghcliCloneRepoFn
	defer func() { ghcliCloneRepoFn = origClone }()

	var cloneCalled bool
	ghcliCloneRepoFn = func(fullName, destPath string) error {
		cloneCalled = true
		t.Logf("ghcliCloneRepoFn called for %s -> %s", fullName, destPath)
		return nil
	}

	// Run import
	imported, err := importReposFn(tmpHome, store, []ghcli.Repo{repo})
	if err != nil {
		t.Fatalf("importReposFn failed: %v", err)
	}

	if len(imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(imported))
	}
	if cloneCalled {
		t.Error("expected clone NOT to be called for valid repo, but it was")
	}
}
