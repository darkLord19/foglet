package ai

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Aider represents the Aider AI tool
type Aider struct{}

func (a *Aider) Name() string {
	return "aider"
}

func (a *Aider) IsAvailable() bool {
	return commandExists("aider")
}

func (a *Aider) Execute(workdir, prompt string) (*Result, error) {
	if !a.IsAvailable() {
		return nil, fmt.Errorf("aider not available")
	}

	// Aider supports CLI execution with --message flag
	// Example: aider --yes --message "implement feature X"
	cmd := exec.Command("aider", "--yes", "--message", prompt)
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
