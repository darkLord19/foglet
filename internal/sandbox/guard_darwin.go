//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"strings"
)

const sandboxExecPath = "/usr/bin/sandbox-exec"

// Wrap re-expresses cmd as a sandbox-exec invocation carrying a generated
// seatbelt profile.
//
// sandbox-exec is deprecated by Apple but remains the only way to restrict a
// child process on macOS without a VM or elevated privileges. When it is
// unavailable or profile setup fails, the command runs unrestricted rather than
// failing: hardening must never be able to break a user's session. Callers
// check Applied when they need to know which happened.
func (g Guard) Wrap(name string, args []string) (Wrapped, error) {
	if Disabled() || len(g.DenyRead) == 0 {
		return passthrough(name, args), nil
	}
	if _, err := os.Stat(sandboxExecPath); err != nil {
		return passthrough(name, args), nil
	}

	profile, err := os.CreateTemp("", "fog-guard-*.sb")
	if err != nil {
		return passthrough(name, args), err
	}
	cleanup := func() { _ = os.Remove(profile.Name()) }

	if _, err := profile.WriteString(g.profileSBPL()); err != nil {
		_ = profile.Close()
		cleanup()
		return passthrough(name, args), err
	}
	if err := profile.Close(); err != nil {
		cleanup()
		return passthrough(name, args), err
	}

	wrappedArgs := make([]string, 0, len(args)+3)
	wrappedArgs = append(wrappedArgs, "-f", profile.Name(), name)
	wrappedArgs = append(wrappedArgs, args...)

	return Wrapped{
		Name:    sandboxExecPath,
		Args:    wrappedArgs,
		Applied: true,
		Cleanup: cleanup,
	}, nil
}

// profileSBPL builds a permissive profile that denies reads of specific paths.
//
// Each path is emitted as both a literal and a subpath rule so files and
// directories are both covered without stat-ing paths that may not exist yet.
func (g Guard) profileSBPL() string {
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(allow default)\n")
	for _, p := range g.DenyRead {
		quoted := quoteSBPL(p)
		fmt.Fprintf(&b, "(deny file-read* (literal %s))\n", quoted)
		fmt.Fprintf(&b, "(deny file-read* (subpath %s))\n", quoted)
	}
	return b.String()
}

// quoteSBPL renders s as a profile string literal.
func quoteSBPL(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
