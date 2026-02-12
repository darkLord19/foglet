package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents global wtx configuration
type Config struct {
	Editor        string `json:"editor"`
	ReuseWindow   bool   `json:"reuse_window"`
	WorktreeDir   string `json:"worktree_dir"`
	AutoStartDev  bool   `json:"auto_start_dev"`
	DefaultBranch string `json:"default_branch"`
	SetupCmd      string `json:"setup_cmd"`    // Command to run after creating worktree
	ValidateCmd   string `json:"validate_cmd"` // Command to validate worktree
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Editor:        "", // Will be detected from $EDITOR
		ReuseWindow:   true,
		WorktreeDir:   "../worktrees",
		AutoStartDev:  false,
		DefaultBranch: "main",
	}
}

// Load reads configuration from disk
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// If config doesn't exist, return defaults
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes configuration to disk
func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	return filepath.Join(home, ".config", "wtx", "config.json"), nil
}

// ConfigDir returns the config directory
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	return filepath.Join(home, ".config", "wtx"), nil
}
