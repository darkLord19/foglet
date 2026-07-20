// Package sandbox applies host-level restrictions to child processes.
//
// Fog executes third-party AI CLIs on the user's machine, and those processes
// inherit the daemon's ambient authority. Without restriction an agent can read
// credentials that have nothing to do with the repository it was asked to work
// on. Guard narrows that surface for every agent invocation, independently of
// whether container-based sandboxing is enabled.
package sandbox

import (
	"os"
	"path/filepath"
	"strings"
)

// disableEnv turns the guard off for a run. Intended for debugging a session
// that misbehaves under restriction.
const disableEnv = "FOG_DISABLE_HOST_GUARD"

// Guard describes paths a child process must not be able to read.
type Guard struct {
	DenyRead []string
}

// Wrapped is a command with a Guard applied.
type Wrapped struct {
	Name string
	Args []string
	// Applied reports whether restrictions were actually installed. It is false
	// on platforms with no implementation, when the guard is disabled, and when
	// setup failed, so callers can distinguish "restricted" from "ran anyway".
	Applied bool
	Cleanup func()
}

func passthrough(name string, args []string) Wrapped {
	return Wrapped{Name: name, Args: args, Cleanup: func() {}}
}

// Disabled reports whether the operator has turned the guard off.
func Disabled() bool {
	return strings.TrimSpace(os.Getenv(disableEnv)) != ""
}

// DefaultGuard denies credentials an AI agent has no reason to read: SSH and
// cloud keys, the gh token, other projects' Claude history, and Fog's own
// master key and database.
//
// fogHome is denied selectively rather than wholesale. Session worktrees live
// under fogHome/repos, so a blanket deny would lock the agent out of the very
// code it was asked to edit.
func DefaultGuard(homeDir, fogHome string) Guard {
	var deny []string
	if strings.TrimSpace(homeDir) != "" {
		deny = append(deny,
			filepath.Join(homeDir, ".ssh"),
			filepath.Join(homeDir, ".aws"),
			filepath.Join(homeDir, ".config", "gh"),
			filepath.Join(homeDir, ".claude.json"),
		)
	}
	if strings.TrimSpace(fogHome) != "" {
		deny = append(deny,
			filepath.Join(fogHome, "master.key"),
			filepath.Join(fogHome, "api.token"),
			filepath.Join(fogHome, "fog.db"),
			filepath.Join(fogHome, "fog.db-wal"),
			filepath.Join(fogHome, "fog.db-shm"),
		)
	}
	return Guard{DenyRead: expandPaths(deny)}
}

// expandPaths returns each path in both its literal and symlink-resolved form.
//
// This matters more than it looks. A seatbelt rule naming an unresolved path
// silently matches nothing — on macOS /tmp is a symlink to /private/tmp, so a
// rule written against the former protects nothing at all and fails open.
func expandPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths)*2)
	out := make([]string, 0, len(paths)*2)

	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		add(filepath.Clean(abs))
		// EvalSymlinks fails on paths that do not exist yet. The literal form
		// added above still covers those once they are created.
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			add(resolved)
		}
	}
	return out
}
