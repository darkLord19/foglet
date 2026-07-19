package ai

import "slices"

import "testing"

func TestGetToolAntigravity(t *testing.T) {
	tool, err := GetTool("antigravity")
	if err != nil {
		t.Fatalf("GetTool returned error: %v", err)
	}
	if tool.Name() != "antigravity" {
		t.Fatalf("unexpected tool name: got %q want %q", tool.Name(), "antigravity")
	}
}

func TestGetToolAntigravityAlias(t *testing.T) {
	tool, err := GetTool("agy")
	if err != nil {
		t.Fatalf("GetTool returned error: %v", err)
	}
	if tool.Name() != "antigravity" {
		t.Fatalf("unexpected tool name: got %q want %q", tool.Name(), "antigravity")
	}
}

func TestGetToolClaudeAlias(t *testing.T) {
	tool, err := GetTool("claude-code")
	if err != nil {
		t.Fatalf("GetTool returned error: %v", err)
	}
	if tool.Name() != "claude" {
		t.Fatalf("unexpected tool name: got %q want %q", tool.Name(), "claude")
	}
}

func TestAvailableToolNamesIncludesAntigravity(t *testing.T) {
	names := AvailableToolNames()
	found := slices.Contains(names, "antigravity")
	if !found {
		t.Fatalf("expected antigravity in available tool names: %v", names)
	}
}
