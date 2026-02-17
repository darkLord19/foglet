package ai

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestCursorAgentCommandPrefersCursorAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses unix-like executable fixtures")
	}

	tempDir := t.TempDir()
	cursorAgentPath := filepath.Join(tempDir, "cursor-agent")
	if err := os.WriteFile(cursorAgentPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write cursor-agent fixture: %v", err)
	}
	agentPath := filepath.Join(tempDir, "agent")
	if err := os.WriteFile(agentPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write agent fixture: %v", err)
	}

	t.Setenv("PATH", tempDir)
	commandPathCache = sync.Map{}

	got := cursorAgentCommand()
	if got != cursorAgentPath {
		t.Fatalf("unexpected command: got %q want %q", got, cursorAgentPath)
	}
}

func TestCursorAgentCommandFallsBackToAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses unix-like executable fixtures")
	}

	tempDir := t.TempDir()
	agentPath := filepath.Join(tempDir, "agent")
	if err := os.WriteFile(agentPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write agent fixture: %v", err)
	}

	t.Setenv("PATH", tempDir)
	commandPathCache = sync.Map{}
	commandPathCache.Store("cursor-agent", "")

	got := cursorAgentCommand()
	if got != agentPath {
		t.Fatalf("unexpected command: got %q want %q", got, agentPath)
	}
}
