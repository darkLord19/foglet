package main

import "testing"

func TestNormalizeScopes(t *testing.T) {
	out := normalizeScopes("app_mentions:read, chat:write, ,commands")
	if len(out) != 3 {
		t.Fatalf("unexpected scope count: %d", len(out))
	}
	if out[0] != "app_mentions:read" || out[1] != "chat:write" || out[2] != "commands" {
		t.Fatalf("unexpected scopes: %#v", out)
	}
}
