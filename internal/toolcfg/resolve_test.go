package toolcfg

import (
	"errors"
	"testing"
)

type fakeStore struct {
	tool  string
	found bool
	err   error
}

func (f fakeStore) GetDefaultTool() (string, bool, error) {
	return f.tool, f.found, f.err
}

func TestResolveToolRequestedWins(t *testing.T) {
	tool, err := ResolveTool("cursor", fakeStore{}, "cli")
	if err != nil {
		t.Fatalf("ResolveTool returned error: %v", err)
	}
	if tool != "cursor" {
		t.Fatalf("tool mismatch: got %q want %q", tool, "cursor")
	}
}

func TestResolveToolUsesDefault(t *testing.T) {
	tool, err := ResolveTool("", fakeStore{tool: "claude", found: true}, "api")
	if err != nil {
		t.Fatalf("ResolveTool returned error: %v", err)
	}
	if tool != "claude" {
		t.Fatalf("tool mismatch: got %q want %q", tool, "claude")
	}
}

func TestResolveToolMissingDefault(t *testing.T) {
	_, err := ResolveTool("", fakeStore{found: false}, "slack")
	if err == nil {
		t.Fatal("expected error when no default tool exists")
	}
}

func TestResolveToolStoreError(t *testing.T) {
	_, err := ResolveTool("", fakeStore{err: errors.New("boom")}, "api")
	if err == nil {
		t.Fatal("expected error when store fails")
	}
}
