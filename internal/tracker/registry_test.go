package tracker

import (
	"errors"
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/task"
)

// Adding a tracker used to mean five edits in internal/api and none here.
// These tests pin the declaration so that stays true in reverse: everything a
// caller needs is reachable from the descriptor.

func TestEveryProviderHasACompleteDescriptor(t *testing.T) {
	providers := Providers()
	if len(providers) == 0 {
		t.Fatal("no providers registered")
	}

	for _, name := range providers {
		d, ok := Describe(name)
		if !ok {
			t.Errorf("Describe(%s) reported not found but Providers() listed it", name)
			continue
		}
		if d.Name != name {
			t.Errorf("%s: descriptor Name = %s", name, d.Name)
		}
		if d.SecretKey == "" {
			t.Errorf("%s: no SecretKey — its token would be unreachable", name)
		}
		if !strings.HasPrefix(d.SecretKey, "tracker."+string(name)+".") {
			t.Errorf("%s: SecretKey %q is not namespaced to the provider", name, d.SecretKey)
		}
		if d.New == nil {
			t.Errorf("%s: no constructor", name)
		}
		for _, key := range d.SettingKeys {
			if !strings.HasPrefix(key, "tracker."+string(name)+".") {
				t.Errorf("%s: setting key %q is not namespaced to the provider", name, key)
			}
		}
	}
}

func TestProviderSecretKeysAreDistinct(t *testing.T) {
	// One provider reading another's token would be a credential leak.
	seen := map[string]task.Provider{}
	for _, name := range Providers() {
		d, _ := Describe(name)
		if other, clash := seen[d.SecretKey]; clash {
			t.Errorf("%s and %s share secret key %q", name, other, d.SecretKey)
		}
		seen[d.SecretKey] = name
	}
}

func TestSecretKeyFor(t *testing.T) {
	if key, ok := SecretKeyFor(task.ProviderLinear); !ok || key != "tracker.linear.token" {
		t.Errorf("SecretKeyFor(linear) = (%q, %v)", key, ok)
	}
	if key, ok := SecretKeyFor(task.ProviderJira); !ok || key != "tracker.jira.token" {
		t.Errorf("SecretKeyFor(jira) = (%q, %v)", key, ok)
	}
	// Local is not a tracker and has no token.
	if _, ok := SecretKeyFor(task.ProviderLocal); ok {
		t.Error("SecretKeyFor(local) reported a secret key")
	}
	if _, ok := SecretKeyFor(task.Provider("nope")); ok {
		t.Error("SecretKeyFor(nope) reported a secret key")
	}
}

func TestNewProviderRejectsUnknownProvider(t *testing.T) {
	_, err := NewProvider(task.ProviderLocal, Credentials{Token: "x"}, DefaultStatusMap())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

func TestNewProviderRejectsMissingToken(t *testing.T) {
	for _, name := range Providers() {
		for _, token := range []string{"", "   "} {
			_, err := NewProvider(name, Credentials{Token: token}, DefaultStatusMap())
			if !errors.Is(err, ErrNotConfigured) {
				t.Errorf("%s with token %q: error = %v, want ErrNotConfigured", name, token, err)
			}
		}
	}
}

func TestNewProviderBuildsLinear(t *testing.T) {
	p, err := NewProvider(task.ProviderLinear, Credentials{
		Token:    "lin_api_key",
		Settings: map[string]string{SettingKey(task.ProviderLinear, "team"): "ENG"},
	}, DefaultStatusMap())
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if p.Name() != task.ProviderLinear {
		t.Errorf("Name() = %s, want linear", p.Name())
	}
}

func TestNewProviderBuildsJira(t *testing.T) {
	p, err := NewProvider(task.ProviderJira, Credentials{
		Token: "jira_token",
		Settings: map[string]string{
			SettingKey(task.ProviderJira, "url"):   "https://acme.atlassian.net",
			SettingKey(task.ProviderJira, "email"): "dev@acme.test",
			SettingKey(task.ProviderJira, "jql"):   "assignee = currentUser()",
		},
	}, DefaultStatusMap())
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if p.Name() != task.ProviderJira {
		t.Errorf("Name() = %s, want jira", p.Name())
	}
}

func TestNewProviderTreatsMissingRequiredConfigAsNotConfigured(t *testing.T) {
	// Jira needs a base URL. Omitting it is "not configured" — the same signal
	// the sync loop already treats as "nothing to do" — not a hard failure that
	// would surface an error to the user for a tracker they never finished
	// setting up. NewJira makes that call; NewProvider must not mask it.
	_, err := NewProvider(task.ProviderJira, Credentials{
		Token:    "jira_token",
		Settings: map[string]string{},
	}, DefaultStatusMap())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured for a Jira provider with no base URL", err)
	}
}

func TestSettingKeyIsNamespaced(t *testing.T) {
	if got := SettingKey(task.ProviderJira, "url"); got != "tracker.jira.url" {
		t.Errorf("SettingKey = %q, want tracker.jira.url", got)
	}
}
