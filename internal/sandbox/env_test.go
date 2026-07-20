package sandbox

import (
	"strings"
	"testing"
)

func envNames(entries []string) map[string]string {
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			out[name] = value
		}
	}
	return out
}

func TestFilterEnvDropsForeignCredentials(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"HOME=/home/example",
		"AWS_SECRET_ACCESS_KEY=aws-secret",
		"AWS_SESSION_TOKEN=aws-token",
		"GITHUB_TOKEN=gh-secret",
		"GH_TOKEN=gh-secret",
		"DATABASE_URL=postgres://user:pw@host/db",
		"STRIPE_SECRET_KEY=stripe-secret",
		"OPENAI_API_KEY=openai-secret",
		// A NODE_ prefix match would have admitted this npm registry credential.
		"NODE_AUTH_TOKEN=npm-secret",
	}

	got := envNames(FilterEnv(input, nil))

	for _, keep := range []string{"PATH", "HOME"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("expected %s to survive filtering", keep)
		}
	}
	for _, drop := range []string{
		"AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "GITHUB_TOKEN", "GH_TOKEN",
		"DATABASE_URL", "STRIPE_SECRET_KEY", "OPENAI_API_KEY", "NODE_AUTH_TOKEN",
	} {
		if _, ok := got[drop]; ok {
			t.Errorf("%s leaked through the filter", drop)
		}
	}
}

func TestFilterEnvKeepsProxyAndCASettings(t *testing.T) {
	input := []string{
		"HTTPS_PROXY=http://proxy:3128",
		"https_proxy=http://proxy:3128",
		"NO_PROXY=localhost",
		"NODE_EXTRA_CA_CERTS=/etc/ssl/corp.pem",
		"SSL_CERT_FILE=/etc/ssl/cert.pem",
		"LC_ALL=en_US.UTF-8",
		"XDG_CONFIG_HOME=/home/example/.config",
	}

	got := envNames(FilterEnv(input, nil))
	if len(got) != len(input) {
		t.Fatalf("expected all proxy/CA/locale vars to survive, got %v", got)
	}
}

// Per-tool scoping is the point of the prefix argument: one agent's key must not
// appear in another agent's environment.
func TestFilterEnvScopesToolCredentials(t *testing.T) {
	input := []string{
		"ANTHROPIC_API_KEY=anthropic-secret",
		"CLAUDE_CONFIG_DIR=/home/example/.claude",
		"CURSOR_API_KEY=cursor-secret",
	}

	claude := envNames(FilterEnv(input, []string{"ANTHROPIC_", "CLAUDE_"}))
	if _, ok := claude["ANTHROPIC_API_KEY"]; !ok {
		t.Error("claude should receive ANTHROPIC_API_KEY")
	}
	if _, ok := claude["CLAUDE_CONFIG_DIR"]; !ok {
		t.Error("claude should receive CLAUDE_CONFIG_DIR")
	}
	if _, ok := claude["CURSOR_API_KEY"]; ok {
		t.Error("claude must not receive CURSOR_API_KEY")
	}

	cursor := envNames(FilterEnv(input, []string{"CURSOR_"}))
	if _, ok := cursor["ANTHROPIC_API_KEY"]; ok {
		t.Error("cursor must not receive ANTHROPIC_API_KEY")
	}
}

// GOOGLE_APPLICATION_CREDENTIALS points at a GCP service account key. No tool
// prefix should admit it, including Antigravity's.
func TestFilterEnvDropsGoogleApplicationCredentials(t *testing.T) {
	input := []string{"GOOGLE_APPLICATION_CREDENTIALS=/home/example/gcp-key.json"}

	got := envNames(FilterEnv(input, []string{"ANTIGRAVITY_", "AGY_", "GEMINI_"}))
	if _, ok := got["GOOGLE_APPLICATION_CREDENTIALS"]; ok {
		t.Error("GOOGLE_APPLICATION_CREDENTIALS leaked to the agent")
	}
}

// nil signals "inherit unchanged" to proc, which is what the escape hatch needs.
func TestFilterEnvReturnsNilWhenDisabled(t *testing.T) {
	t.Setenv(disableEnv, "1")

	if got := FilterEnv([]string{"AWS_SECRET_ACCESS_KEY=secret"}, nil); got != nil {
		t.Fatalf("expected nil to signal inheritance, got %v", got)
	}
}

func TestFilterEnvSkipsMalformedEntries(t *testing.T) {
	got := FilterEnv([]string{"NOT_AN_ASSIGNMENT", "PATH=/usr/bin"}, nil)
	if len(got) != 1 || got[0] != "PATH=/usr/bin" {
		t.Fatalf("expected only the well-formed entry, got %v", got)
	}
}

func TestFilterEnvMatchesNamesCaseInsensitively(t *testing.T) {
	got := envNames(FilterEnv([]string{"http_proxy=http://proxy:3128"}, nil))
	if _, ok := got["http_proxy"]; !ok {
		t.Error("lowercase proxy spelling should be allowed by the uppercase entry")
	}
}
