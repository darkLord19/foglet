package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/proc"
)

// ClaudeCode represents the Claude Code AI tool
type ClaudeCode struct{}

func (c *ClaudeCode) Name() string {
	return "claude"
}

func (c *ClaudeCode) IsAvailable() bool {
	return commandExists("claude") || commandExists("claude-code")
}

func (c *ClaudeCode) Execute(ctx context.Context, workdir, prompt string) (*Result, error) {
	if !c.IsAvailable() {
		return nil, fmt.Errorf("claude not available")
	}

	cmdName := "claude"
	if !commandExists("claude") {
		cmdName = "claude-code"
	}

	// Claude Code supports CLI execution with the 'chat' command
	// Example: claude chat "implement feature X"
	output, err := proc.Run(ctx, workdir, cmdName, "chat", prompt)
	trimmed := strings.TrimSpace(string(output))

	result := &Result{
		Success: err == nil,
		Output:  trimmed,
	}

	if err != nil {
		result.Error = err
	}

	return result, err
}
