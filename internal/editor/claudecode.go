package editor

import (
	"os/exec"
)

// ClaudeCode represents Claude Code editor
type ClaudeCode struct{}

func (c *ClaudeCode) Name() string {
	return "claudecode"
}

func (c *ClaudeCode) IsAvailable() bool {
	// Claude Code might be available as 'claude' or 'claude-code'
	return commandExists("claude") || commandExists("claude-code")
}

func (c *ClaudeCode) Open(path string, reuse bool) error {
	// Try 'claude' first, then 'claude-code'
	cmdName := "claude"
	if !commandExists("claude") {
		cmdName = "claude-code"
	}
	
	args := []string{path}
	// Claude Code may support reuse window in the future
	// For now, just open the path
	
	cmd := exec.Command(cmdName, args...)
	return cmd.Start()
}
