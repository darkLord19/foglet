// Package binpath provides a shared fallbackBinDirs helper used by ai and ghcli.
package binpath

import (
	"os"
	"path/filepath"
	"strings"
)

// FallbackBinDirs returns a deduplicated list of common binary directories
// beyond the standard PATH. It includes Homebrew, local user, and common
// language-runtime bin directories.
func FallbackBinDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/opt/homebrew/sbin",
		"/usr/local/sbin",
		"/usr/bin",
		"/bin",
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".npm-global", "bin"),
			filepath.Join(home, "Library", "pnpm"),
		)
		if matches, err := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin")); err == nil {
			dirs = append(dirs, matches...)
		}
	}

	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}
