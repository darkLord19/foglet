package main

import "testing"

func TestChooseDefaultToolRequested(t *testing.T) {
	tool, err := chooseDefaultTool([]string{"cursor", "claude"}, "claude")
	if err != nil {
		t.Fatalf("chooseDefaultTool returned error: %v", err)
	}
	if tool != "claude" {
		t.Fatalf("tool mismatch: got %q want %q", tool, "claude")
	}
}

func TestChooseDefaultToolRequestedUnavailable(t *testing.T) {
	_, err := chooseDefaultTool([]string{"cursor"}, "claude")
	if err == nil {
		t.Fatal("expected error for unavailable tool")
	}
}

func TestChooseDefaultToolSingleOption(t *testing.T) {
	tool, err := chooseDefaultTool([]string{"cursor"}, "")
	if err != nil {
		t.Fatalf("chooseDefaultTool returned error: %v", err)
	}
	if tool != "cursor" {
		t.Fatalf("tool mismatch: got %q want %q", tool, "cursor")
	}
}
