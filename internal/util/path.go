package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands ~ to home directory and resolves relative paths
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	
	// Expand ~
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}
	
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %w", err)
	}
	
	return absPath, nil
}

// PathExists checks if a path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir checks if a path is a directory
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// WorktreeNameFromPath extracts the worktree name from a path
func WorktreeNameFromPath(path string) string {
	return filepath.Base(path)
}

// SanitizePath prevents path traversal attacks
func SanitizePath(base, user string) (string, error) {
	cleaned := filepath.Clean(user)
	full := filepath.Join(base, cleaned)
	
	// Ensure the full path is still under base
	if !strings.HasPrefix(full, base) {
		return "", fmt.Errorf("invalid path: %s", user)
	}
	
	return full, nil
}
