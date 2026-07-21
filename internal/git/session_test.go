package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestIsDirty(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	dirty, err := g.IsDirty()
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("a freshly committed repo should be clean")
	}

	write(t, dir, "new.txt", "content")
	dirty, err = g.IsDirty()
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if !dirty {
		t.Error("an untracked file should make the worktree dirty")
	}
}

func TestStageAllAndCommit(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	write(t, dir, "feature.txt", "work")
	if err := g.StageAll(); err != nil {
		t.Fatalf("StageAll: %v", err)
	}

	sha, err := g.Commit("feat: add a feature")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("Commit returned %q, want a 40-character SHA", sha)
	}

	head, err := g.HeadSHA()
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if head != sha {
		t.Errorf("HeadSHA = %q, want the committed SHA %q", head, sha)
	}

	dirty, err := g.IsDirty()
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("worktree should be clean after committing everything")
	}
}

func TestCommitRejectsEmptyMessage(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)
	write(t, dir, "feature.txt", "work")
	if err := g.StageAll(); err != nil {
		t.Fatalf("StageAll: %v", err)
	}

	if _, err := g.Commit("   "); err == nil {
		t.Fatal("Commit succeeded with an empty message, want an error")
	}
}

func TestStagedChanges(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	write(t, dir, "feature.txt", "hello world\n")
	if err := g.StageAll(); err != nil {
		t.Fatalf("StageAll: %v", err)
	}

	diff, err := g.StagedChanges()
	if err != nil {
		t.Fatalf("StagedChanges: %v", err)
	}
	if !strings.Contains(diff.NameStatus, "feature.txt") {
		t.Errorf("NameStatus = %q, want it to mention feature.txt", diff.NameStatus)
	}
	if !strings.Contains(diff.Stat, "feature.txt") {
		t.Errorf("Stat = %q, want it to mention feature.txt", diff.Stat)
	}
	if !strings.Contains(diff.Patch, "hello world") {
		t.Errorf("Patch = %q, want it to contain the added line", diff.Patch)
	}
}

func TestStagedChangesEmptyWhenNothingStaged(t *testing.T) {
	g := New(initRepo(t))
	diff, err := g.StagedChanges()
	if err != nil {
		t.Fatalf("StagedChanges: %v", err)
	}
	if diff.NameStatus != "" || diff.Patch != "" {
		t.Errorf("expected an empty diff, got %+v", diff)
	}
}

func TestPushFailsWithoutRemote(t *testing.T) {
	g := New(initRepo(t))
	if err := g.Push("main", false); err == nil {
		t.Fatal("Push succeeded without a remote, want an error")
	}
}

// The point of routing internal/git through internal/proc: a cancelled context
// stops the git process instead of leaking it.
func TestCommandsRespectContextCancellation(t *testing.T) {
	dir := initRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	if _, err := New(dir).WithContext(ctx).HeadSHA(); err == nil {
		t.Fatal("expected a cancelled context to fail the command")
	}
}

func TestCommandsRespectContextDeadline(t *testing.T) {
	dir := initRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	if _, err := New(dir).WithContext(ctx).HeadSHA(); err == nil {
		t.Fatal("expected an expired deadline to fail the command")
	}
}

func TestWithContextDoesNotMutateReceiver(t *testing.T) {
	dir := initRepo(t)
	base := New(dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = base.WithContext(ctx)

	// The original must still be usable — WithContext returns a copy.
	if _, err := base.HeadSHA(); err != nil {
		t.Fatalf("WithContext mutated the receiver: %v", err)
	}
}

func TestNilContextFallsBackToBackground(t *testing.T) {
	dir := initRepo(t)
	//nolint:staticcheck // deliberately passing nil to pin the fallback
	if _, err := New(dir).WithContext(nil).HeadSHA(); err != nil {
		t.Fatalf("a nil context should fall back to Background: %v", err)
	}
}

func TestZeroValueGitDoesNotPanic(t *testing.T) {
	// A Git built as a literal has no context; context() must tolerate it.
	g := &Git{repoPath: initRepo(t)}
	if _, err := g.HeadSHA(); err != nil {
		t.Fatalf("zero-value Git failed: %v", err)
	}
}
