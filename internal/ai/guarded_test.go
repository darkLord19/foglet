package ai

import (
	"context"
	"os"
	"strings"
	"testing"
)

// A tool with no entry in toolEnvPrefixes silently loses its own credentials,
// which would look like an auth failure rather than a configuration gap. Fail
// here instead, when a new adapter is registered.
func TestEveryRegisteredToolHasEnvPrefixes(t *testing.T) {
	for _, name := range AvailableToolNames() {
		if len(toolEnvPrefixes[name]) == 0 {
			t.Errorf("tool %q has no entry in toolEnvPrefixes; its credentials would be filtered out", name)
		}
	}
}

func TestToolEnvPrefixKeysAreRegisteredTools(t *testing.T) {
	registered := make(map[string]struct{})
	for _, name := range AvailableToolNames() {
		registered[name] = struct{}{}
	}
	for name := range toolEnvPrefixes {
		if _, ok := registered[name]; !ok {
			t.Errorf("toolEnvPrefixes has key %q which is not a registered tool name", name)
		}
	}
}

// End-to-end proof that filtering reaches the child process, rather than only
// the slice we hand to exec.
func TestRunGuardedStreamingFiltersChildEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a child process; skipped under -short")
	}
	if _, err := os.Stat("/usr/bin/env"); err != nil {
		t.Skipf("/usr/bin/env unavailable: %v", err)
	}

	t.Setenv("AWS_SECRET_ACCESS_KEY", "aws-canary")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-canary")
	t.Setenv("CURSOR_API_KEY", "cursor-canary")

	var out strings.Builder
	_, err := runGuardedStreaming(
		context.Background(),
		"claude",
		t.TempDir(),
		"/usr/bin/env",
		func(chunk []byte) { out.Write(chunk) },
		nil,
	)
	if err != nil {
		t.Fatalf("running env failed: %v", err)
	}

	got := out.String()
	if strings.Contains(got, "aws-canary") {
		t.Error("AWS credential reached the agent process")
	}
	if strings.Contains(got, "cursor-canary") {
		t.Error("Cursor credential reached a Claude process")
	}
	if !strings.Contains(got, "anthropic-canary") {
		t.Error("Claude's own credential was stripped from its environment")
	}
	if !strings.Contains(got, "PATH=") {
		t.Error("PATH missing from child environment")
	}
}
