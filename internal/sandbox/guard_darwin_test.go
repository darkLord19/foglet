//go:build darwin

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func skipWithoutSandboxExec(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("spawns sandbox-exec; skipped under -short")
	}
	if _, err := os.Stat(sandboxExecPath); err != nil {
		t.Skipf("sandbox-exec unavailable: %v", err)
	}
}

// The end-to-end check that the whole slice exists for: a denied file must not
// be readable, while unrelated files stay readable.
//
// t.TempDir lives under /var/folders on macOS, and /var is a symlink to
// /private/var — so this also exercises the symlink resolution that a rule
// written against the unresolved path would silently fail to enforce.
func TestWrapEnforcesDenyRead(t *testing.T) {
	skipWithoutSandboxExec(t)

	dir := t.TempDir()
	secret := filepath.Join(dir, "master.key")
	if err := os.WriteFile(secret, []byte("CANARY"), 0o600); err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(dir, "allowed.txt")
	if err := os.WriteFile(allowed, []byte("VISIBLE"), 0o600); err != nil {
		t.Fatal(err)
	}

	guard := Guard{DenyRead: expandPaths([]string{secret})}

	denied, err := guard.Wrap("/bin/cat", []string{secret})
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Cleanup()
	if !denied.Applied {
		t.Fatal("guard reported not applied")
	}

	out, err := exec.Command(denied.Name, denied.Args...).CombinedOutput()
	if err == nil {
		t.Fatalf("expected denied read to fail, got output %q", out)
	}
	if strings.Contains(string(out), "CANARY") {
		t.Fatalf("secret contents leaked through guard: %q", out)
	}

	permitted, err := guard.Wrap("/bin/cat", []string{allowed})
	if err != nil {
		t.Fatal(err)
	}
	defer permitted.Cleanup()

	out, err = exec.Command(permitted.Name, permitted.Args...).CombinedOutput()
	if err != nil {
		t.Fatalf("unrelated read failed under guard: %v (%q)", err, out)
	}
	if !strings.Contains(string(out), "VISIBLE") {
		t.Fatalf("expected allowed file contents, got %q", out)
	}
}

// A malformed profile makes sandbox-exec fail and write to stderr, which
// proc.RunStreaming merges into the agent's output stream — that both corrupts
// parsed output and can trigger the adapters' unsupported-flag retry path,
// re-running the agent at full token cost. Assert the real profile is accepted
// and produces no extra output.
func TestDefaultGuardProfileIsAcceptedAndQuiet(t *testing.T) {
	skipWithoutSandboxExec(t)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	guard := DefaultGuard(homeDir, filepath.Join(homeDir, ".fog"))

	wrapped, err := guard.Wrap("/bin/echo", []string{"ok"})
	if err != nil {
		t.Fatal(err)
	}
	defer wrapped.Cleanup()

	out, err := exec.Command(wrapped.Name, wrapped.Args...).CombinedOutput()
	if err != nil {
		t.Fatalf("sandbox-exec rejected the generated profile: %v (%q)", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("guard polluted the output stream: %q", out)
	}
}

// Verifies the guard against this machine's real Fog home rather than a
// fixture, which is the claim that actually matters: an agent cannot read the
// key that decrypts stored GitHub and Slack tokens. Skips when Fog has not been
// set up locally, so CI stays green.
func TestDefaultGuardBlocksRealFogSecrets(t *testing.T) {
	skipWithoutSandboxExec(t)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	fogHome := filepath.Join(homeDir, ".fog")
	guard := DefaultGuard(homeDir, fogHome)

	for _, name := range []string{"master.key", "api.token", "fog.db"} {
		target := filepath.Join(fogHome, name)
		if _, err := os.Stat(target); err != nil {
			t.Logf("skipping %s: not present", name)
			continue
		}

		wrapped, err := guard.Wrap("/bin/cat", []string{target})
		if err != nil {
			t.Fatal(err)
		}

		out, err := exec.Command(wrapped.Name, wrapped.Args...).CombinedOutput()
		wrapped.Cleanup()

		// Never echo `out` on failure: when the guard is not enforcing, it holds
		// the secret this test exists to protect, and test logs get captured.
		if err == nil {
			t.Errorf("%s was readable under the guard (%d bytes recovered)", name, len(out))
			continue
		}
		if !strings.Contains(string(out), "Operation not permitted") {
			t.Errorf("%s: expected a permission denial, got a different error", name)
		}
	}
}

func TestWrapCleanupRemovesProfile(t *testing.T) {
	skipWithoutSandboxExec(t)

	guard := Guard{DenyRead: expandPaths([]string{filepath.Join(t.TempDir(), "secret")})}
	wrapped, err := guard.Wrap("/bin/echo", []string{"hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !wrapped.Applied {
		t.Fatal("guard reported not applied")
	}

	profilePath := wrapped.Args[1]
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("profile missing before cleanup: %v", err)
	}

	wrapped.Cleanup()
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Fatalf("profile still present after cleanup: %v", err)
	}
}
