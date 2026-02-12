package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	fogHomeEnv = "FOG_HOME"
)

// FogHome returns Fog's home directory. FOG_HOME overrides the default ~/.fog path.
func FogHome() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(fogHomeEnv)); custom != "" {
		return custom, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home: %w", err)
	}
	return filepath.Join(userHome, ".fog"), nil
}

// ManagedReposDir returns the default directory for managed repositories.
func ManagedReposDir(fogHome string) string {
	return filepath.Join(fogHome, "repos")
}
