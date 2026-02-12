package ai

import (
	"fmt"
	"os/exec"
)

// Tool represents an AI coding tool
type Tool interface {
	Name() string
	IsAvailable() bool
	Execute(workdir, prompt string) (*Result, error)
}

// Result contains the AI execution result
type Result struct {
	Success bool
	Output  string
	Error   error
}

// GetTool returns an AI tool by name
func GetTool(name string) (Tool, error) {
	switch name {
	case "cursor":
		return &Cursor{}, nil
	case "claude", "claude-code":
		return &ClaudeCode{}, nil
	case "aider":
		return &Aider{}, nil
	default:
		return nil, fmt.Errorf("unknown AI tool: %s", name)
	}
}

// DetectTool finds an available AI tool
func DetectTool(preferred string) (Tool, error) {
	tools := []Tool{
		&Cursor{},
		&ClaudeCode{},
		&Aider{},
	}

	// Try preferred first
	if preferred != "" {
		tool, err := GetTool(preferred)
		if err == nil && tool.IsAvailable() {
			return tool, nil
		}
	}

	// Fall back to first available
	for _, tool := range tools {
		if tool.IsAvailable() {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("no AI tool available")
}

// commandExists checks if a command is available
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
