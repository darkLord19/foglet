package api

import (
	"errors"
	"testing"

	"github.com/darkLord19/foglet/internal/task"
	"github.com/darkLord19/foglet/internal/tracker"
)

// internal/api/tracker.go had no tests and was the last file in the codebase at
// 0% coverage that decides which secret gets read. buildProvider and
// trackerConfig now delegate the provider-specific knowledge to
// internal/tracker; these cover the wiring that remains here.

func TestBuildProviderUnconfiguredIsNotConfigured(t *testing.T) {
	srv := newTestServer(t)
	_, _, err := srv.buildProvider()
	if !errors.Is(err, tracker.ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured with no tracker set up", err)
	}
}

func TestBuildProviderMissingTokenIsNotConfigured(t *testing.T) {
	srv := newTestServer(t)
	// Provider selected, settings present, but no token stored.
	mustSet(t, srv, "tracker.provider", string(task.ProviderLinear))
	mustSet(t, srv, "tracker.linear.team", "ENG")

	_, _, err := srv.buildProvider()
	if !errors.Is(err, tracker.ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured with no token", err)
	}
}

func TestBuildProviderReadsTheDescriptorsSecretKey(t *testing.T) {
	srv := newTestServer(t)
	mustSet(t, srv, "tracker.provider", string(task.ProviderLinear))
	mustSet(t, srv, "tracker.linear.team", "ENG")
	// The token must be stored under exactly the key the descriptor declares.
	key, _ := tracker.SecretKeyFor(task.ProviderLinear)
	if err := srv.stateStore.SaveSecret(key, "lin_token"); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}

	provider, _, err := srv.buildProvider()
	if err != nil {
		t.Fatalf("buildProvider: %v", err)
	}
	if provider == nil || provider.Name() != task.ProviderLinear {
		t.Fatalf("provider = %v, want a linear provider", provider)
	}
}

func TestTrackerConfigReportsTokenPresenceWithoutEchoingIt(t *testing.T) {
	srv := newTestServer(t)
	mustSet(t, srv, "tracker.provider", string(task.ProviderJira))
	mustSet(t, srv, "tracker.jira.url", "https://acme.atlassian.net")

	cfg, err := srv.trackerConfig()
	if err != nil {
		t.Fatalf("trackerConfig: %v", err)
	}
	if cfg.HasToken {
		t.Error("HasToken is true before any token was stored")
	}

	key, _ := tracker.SecretKeyFor(task.ProviderJira)
	if err := srv.stateStore.SaveSecret(key, "jira_token"); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}

	cfg, err = srv.trackerConfig()
	if err != nil {
		t.Fatalf("trackerConfig: %v", err)
	}
	if !cfg.HasToken {
		t.Error("HasToken is false after a token was stored")
	}
}

func TestTrackerConfigDefaultsToLocal(t *testing.T) {
	srv := newTestServer(t)
	cfg, err := srv.trackerConfig()
	if err != nil {
		t.Fatalf("trackerConfig: %v", err)
	}
	if cfg.Provider != string(task.ProviderLocal) {
		t.Errorf("Provider = %q, want local when nothing is configured", cfg.Provider)
	}
	if cfg.HasToken {
		t.Error("local provider reported a token")
	}
}

func mustSet(t *testing.T, srv *Server, key, value string) {
	t.Helper()
	if err := srv.stateStore.SetSetting(key, value); err != nil {
		t.Fatalf("SetSetting(%s): %v", key, err)
	}
}
