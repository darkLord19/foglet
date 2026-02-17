package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWorktreePathWithNameDetectsDefaultBranch(t *testing.T) {
	repo := initGitRepo(t, "master")

	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".config", "wtx", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	// Intentionally set DefaultBranch to a branch that does not exist.
	if err := os.WriteFile(cfgPath, []byte(`{"worktree_dir":"../worktrees","default_branch":"main"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	r, err := New(repo, t.TempDir())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	wtPath, err := r.createWorktreePathWithName(repo, "feature", "feature")
	if err != nil {
		t.Fatalf("createWorktreePathWithName returned error: %v", err)
	}

	expected := filepath.Clean(filepath.Join(repo, "..", "worktrees", "feature"))
	gotPath := filepath.Clean(wtPath)
	if eval, err := filepath.EvalSymlinks(expected); err == nil {
		expected = eval
	}
	if eval, err := filepath.EvalSymlinks(gotPath); err == nil {
		gotPath = eval
	}
	if gotPath != expected {
		t.Fatalf("path mismatch: got %q want %q", gotPath, expected)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree path to exist: %v", err)
	}

	g := gitCmdRepo(t, repo)
	if !gitBranchExists(g, "feature") {
		t.Fatal("expected feature branch to exist after worktree creation")
	}
}

func TestNewAllowsNonRepoPath(t *testing.T) {
	storeDir := t.TempDir()
	nonRepo := t.TempDir()

	r, err := New(nonRepo, storeDir)
	if err != nil {
		t.Fatalf("New returned error for non-repo path: %v", err)
	}
	if r == nil {
		t.Fatal("expected runner instance")
	}
}

func initGitRepo(t *testing.T, defaultBranch string) string {
	t.Helper()

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "fog-test@example.com")
	runGit(t, repo, "config", "user.name", "fog test")

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	if strings.TrimSpace(defaultBranch) == "" {
		defaultBranch = "main"
	}
	runGit(t, repo, "branch", "-M", defaultBranch)

	return repo
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func gitCmdRepo(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "branch", "--list", "--format=%(refname:short)").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list failed: %v\n%s", err, string(out))
	}
	return string(out)
}

func gitBranchExists(branchListOutput string, branch string) bool {
	for _, line := range strings.Split(branchListOutput, "\n") {
		if strings.TrimSpace(line) == branch {
			return true
		}
	}
	return false
}
