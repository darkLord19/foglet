package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contains(list []string, want string) bool {
	for _, got := range list {
		if got == want {
			return true
		}
	}
	return false
}

func TestDefaultGuardDeniesCredentialPaths(t *testing.T) {
	home := filepath.FromSlash("/home/example")
	fogHome := filepath.Join(home, ".fog")
	guard := DefaultGuard(home, fogHome)

	for _, want := range []string{
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".aws"),
		filepath.Join(home, ".config", "gh"),
		filepath.Join(home, ".claude.json"),
		filepath.Join(fogHome, "master.key"),
		filepath.Join(fogHome, "fog.db"),
	} {
		if !contains(guard.DenyRead, want) {
			t.Errorf("DenyRead missing %q; got %v", want, guard.DenyRead)
		}
	}
}

// Session worktrees live under FOG_HOME/repos. Denying FOG_HOME wholesale would
// lock the agent out of the code it was asked to edit, so assert no deny entry
// is an ancestor of the repos directory.
func TestDefaultGuardDoesNotCoverWorktreeRoot(t *testing.T) {
	home := filepath.FromSlash("/home/example")
	fogHome := filepath.Join(home, ".fog")
	reposDir := filepath.Join(fogHome, "repos")

	for _, denied := range DefaultGuard(home, fogHome).DenyRead {
		if denied == fogHome {
			t.Fatalf("DenyRead contains FOG_HOME %q, which would block every session worktree", denied)
		}
		if strings.HasPrefix(reposDir, denied+string(filepath.Separator)) {
			t.Fatalf("DenyRead entry %q is an ancestor of the repos dir %q", denied, reposDir)
		}
	}
}

func TestDefaultGuardWithEmptyInputs(t *testing.T) {
	if guard := DefaultGuard("", ""); len(guard.DenyRead) != 0 {
		t.Fatalf("expected no deny entries, got %v", guard.DenyRead)
	}
}

// A profile rule naming an unresolved symlink silently matches nothing, which
// fails open. expandPaths must emit the resolved form alongside the literal.
func TestExpandPathsIncludesResolvedForm(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got := expandPaths([]string{link})
	if !contains(got, link) {
		t.Errorf("missing literal form %q in %v", link, got)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, resolved) {
		t.Errorf("missing resolved form %q in %v", resolved, got)
	}
}

func TestExpandPathsDedupesAndCleans(t *testing.T) {
	base := filepath.Join(t.TempDir(), "missing")
	got := expandPaths([]string{base, base, base + string(filepath.Separator)})
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped entry, got %v", got)
	}
	if got[0] != filepath.Clean(base) {
		t.Fatalf("expected %q, got %q", filepath.Clean(base), got[0])
	}
}

func TestWrapPassthroughWhenGuardEmpty(t *testing.T) {
	wrapped, err := Guard{}.Wrap("/bin/echo", []string{"hi"})
	if err != nil {
		t.Fatal(err)
	}
	defer wrapped.Cleanup()

	if wrapped.Applied {
		t.Error("empty guard should not report Applied")
	}
	if wrapped.Name != "/bin/echo" || len(wrapped.Args) != 1 || wrapped.Args[0] != "hi" {
		t.Errorf("command was rewritten: %q %v", wrapped.Name, wrapped.Args)
	}
}

func TestWrapPassthroughWhenDisabled(t *testing.T) {
	t.Setenv(disableEnv, "1")

	guard := Guard{DenyRead: []string{filepath.FromSlash("/home/example/.ssh")}}
	wrapped, err := guard.Wrap("/bin/echo", []string{"hi"})
	if err != nil {
		t.Fatal(err)
	}
	defer wrapped.Cleanup()

	if wrapped.Applied {
		t.Error("disabled guard should not report Applied")
	}
	if wrapped.Name != "/bin/echo" {
		t.Errorf("expected passthrough, got %q", wrapped.Name)
	}
}
