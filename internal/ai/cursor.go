package ai

import (
	"fmt"
	"os/exec"
)

// Cursor represents the Cursor AI tool
type Cursor struct{}

func (c *Cursor) Name() string {
	return "cursor"
}

func (c *Cursor) IsAvailable() bool {
	return commandExists("cursor")
}

func (c *Cursor) Execute(workdir, prompt string) (*Result, error) {
	if !c.IsAvailable() {
		return nil, fmt.Errorf("cursor not available")
	}

	// Cursor doesn't have a direct CLI for AI execution yet
	// This is a placeholder for when Cursor CLI supports this
	// For now, we can open the project and the user can use Cursor's AI

	result := &Result{
		Success: true,
		Output:  "Cursor does not support CLI-based AI execution yet. Opening project in Cursor.",
	}

	// Open in Cursor
	cmd := exec.Command("cursor", workdir)
	if err := cmd.Start(); err != nil {
		result.Success = false
		result.Error = err
		return result, err
	}

	return result, nil
}
