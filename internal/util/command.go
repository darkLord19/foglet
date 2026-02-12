package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommandResult contains the result of a command execution
type CommandResult struct {
	Output   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// RunCommand executes a shell command in the given directory
func RunCommand(command, workdir string) *CommandResult {
	start := time.Now()
	result := &CommandResult{}

	// Parse command (simple split on spaces - not shell-safe but works for common cases)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		result.Error = fmt.Errorf("empty command")
		return result
	}

	// Create command
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workdir

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run
	err := cmd.Run()
	result.Duration = time.Since(start)

	// Combine stdout and stderr
	result.Output = stdout.String()
	if stderr.Len() > 0 {
		result.Output += "\n" + stderr.String()
	}

	if err != nil {
		result.Error = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// RunCommandWithTimeout executes a command with a timeout
func RunCommandWithTimeout(command, workdir string, timeout time.Duration) *CommandResult {
	// TODO: Implement timeout support
	// For now, just call RunCommand
	return RunCommand(command, workdir)
}
