package ai

import (
	"bytes"
	"fmt"
	"os/exec"
)

// ClaudeCode represents the Claude Code AI tool
type ClaudeCode struct{}

func (c *ClaudeCode) Name() string {
	return "claude"
}

func (c *ClaudeCode) IsAvailable() bool {
	return commandExists("claude") || commandExists("claude-code")
}

func (c *ClaudeCode) Execute(workdir, prompt string) (*Result, error) {
	if !c.IsAvailable() {
		return nil, fmt.Errorf("claude not available")
	}

	cmdName := "claude"
	if !commandExists("claude") {
		cmdName = "claude-code"
	}

	// Claude Code supports CLI execution with the 'chat' command
	// Example: claude chat "implement feature X"
	cmd := exec.Command(cmdName, "chat", prompt)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &Result{
		Success: err == nil,
		Output:  stdout.String(),
	}

	if err != nil {
		result.Error = err
		result.Output += "\nError: " + stderr.String()
	}

	return result, err
}
