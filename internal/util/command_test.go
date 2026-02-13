package util

import (
	"strings"
	"testing"
	"time"
)

func TestRunCommand(t *testing.T) {
	// Successful command
	result := RunCommand("echo hello", "")
	if result.Error != nil {
		t.Errorf("RunCommand failed: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("Unexpected output: got %q, want %q", result.Output, "hello")
	}
	if result.ExitCode != 0 {
		t.Errorf("Unexpected exit code: got %d, want 0", result.ExitCode)
	}

	// Failing command
	result = RunCommand("ls non_existent_file_likely_does_not_exist", "")
	if result.Error == nil {
		t.Error("Expected error for non-existent file, but got nil")
	}
	if result.ExitCode == 0 {
		t.Errorf("Expected non-zero exit code, but got %d", result.ExitCode)
	}
}

func TestRunCommandEmpty(t *testing.T) {
	result := RunCommand("", "")
	if result.Error == nil {
		t.Error("Expected error for empty command, but got nil")
	}
	if result.Error.Error() != "empty command" {
		t.Errorf("Unexpected error message: %v", result.Error)
	}
}

func TestRunCommandWithTimeout(t *testing.T) {
	// Success within timeout
	result := RunCommandWithTimeout("echo hello", "", 1*time.Second)
	if result.Error != nil {
		t.Errorf("RunCommandWithTimeout failed: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("Unexpected output: got %q, want %q", result.Output, "hello")
	}

	// Timeout
	start := time.Now()
	result = RunCommandWithTimeout("sleep 2", "", 500*time.Millisecond)
	duration := time.Since(start)

	if duration > 1*time.Second {
		t.Errorf("RunCommandWithTimeout took too long: %v", duration)
	}
	if result.Error == nil {
		t.Error("Expected error due to timeout, but got nil")
	}
	// Check if the error is context deadline exceeded
	if !strings.Contains(result.Error.Error(), "context deadline exceeded") && !strings.Contains(result.Error.Error(), "signal: killed") {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}
