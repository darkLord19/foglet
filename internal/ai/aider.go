package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/proc"
)

// Aider represents the Aider AI tool
type Aider struct{}

func (a *Aider) Name() string {
	return "aider"
}

func (a *Aider) IsAvailable() bool {
	return commandExists("aider")
}

func (a *Aider) Execute(ctx context.Context, workdir, prompt string) (*Result, error) {
	if !a.IsAvailable() {
		return nil, fmt.Errorf("aider not available")
	}

	// Aider supports CLI execution with --message flag
	// Example: aider --yes --message "implement feature X"
	output, err := proc.Run(ctx, workdir, "aider", "--yes", "--message", prompt)
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
