package tracker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/darkLord19/foglet/internal/task"
)

// Provider construction, owned by this package rather than by its callers.
//
// The Provider interface was already a real seam — Linear and Jira are
// genuinely different implementations, not parallel copies. Construction was
// not: internal/api held a switch that knew each provider's credential shape,
// which secret key its token lived under, and which settings keys carried its
// configuration. Adding a third tracker meant five edits in the HTTP package
// and none here.
//
// A Descriptor moves that knowledge next to the adapter it describes.

// Credentials carries the secret plus the provider-specific settings needed to
// construct an adapter. Fields not relevant to a provider are ignored.
type Credentials struct {
	// Token is the API key or personal access token.
	Token string
	// Settings holds the provider's non-secret configuration, keyed by the
	// Descriptor's SettingKeys.
	Settings map[string]string
}

// Descriptor declares everything Fog needs to know about one tracker.
type Descriptor struct {
	// Name is the canonical provider identity.
	Name task.Provider

	// SecretKey is where this provider's token is stored.
	SecretKey string

	// SettingKeys are the non-secret settings this provider reads, in the order
	// a UI should present them. Each is a fully-qualified settings key.
	SettingKeys []string

	// New builds an adapter from resolved credentials.
	New func(creds Credentials, statuses StatusMap) (Provider, error)
}

// SettingKey returns the fully-qualified settings key for a provider field.
func SettingKey(provider task.Provider, field string) string {
	return "tracker." + string(provider) + "." + field
}

var descriptors = map[task.Provider]Descriptor{
	task.ProviderLinear: {
		Name:      task.ProviderLinear,
		SecretKey: "tracker.linear.token",
		SettingKeys: []string{
			SettingKey(task.ProviderLinear, "team"),
		},
		New: func(creds Credentials, statuses StatusMap) (Provider, error) {
			return NewLinear(
				creds.Token,
				creds.Settings[SettingKey(task.ProviderLinear, "team")],
				statuses,
			)
		},
	},
	task.ProviderJira: {
		Name:      task.ProviderJira,
		SecretKey: "tracker.jira.token",
		SettingKeys: []string{
			SettingKey(task.ProviderJira, "url"),
			SettingKey(task.ProviderJira, "email"),
			SettingKey(task.ProviderJira, "jql"),
		},
		New: func(creds Credentials, statuses StatusMap) (Provider, error) {
			return NewJira(
				creds.Settings[SettingKey(task.ProviderJira, "url")],
				creds.Settings[SettingKey(task.ProviderJira, "email")],
				creds.Token,
				creds.Settings[SettingKey(task.ProviderJira, "jql")],
				statuses,
			)
		},
	},
}

// Describe returns the descriptor for a provider.
func Describe(provider task.Provider) (Descriptor, bool) {
	d, ok := descriptors[provider]
	return d, ok
}

// Providers returns every supported provider name, in a stable order.
func Providers() []task.Provider {
	out := make([]task.Provider, 0, len(descriptors))
	for name := range descriptors {
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SecretKeyFor returns where a provider's token is stored.
//
// This decision used to be made by an inline two-line conditional in three
// separate places in internal/api, twice with mismatched types.
func SecretKeyFor(provider task.Provider) (string, bool) {
	d, ok := descriptors[provider]
	if !ok {
		return "", false
	}
	return d.SecretKey, true
}

// NewProvider builds the adapter for a provider from its credentials.
//
// It returns ErrNotConfigured for an unknown provider or a missing token, which
// is the same signal the sync loop already treats as "nothing to do".
func NewProvider(provider task.Provider, creds Credentials, statuses StatusMap) (Provider, error) {
	d, ok := descriptors[provider]
	if !ok {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(creds.Token) == "" {
		return nil, ErrNotConfigured
	}
	p, err := d.New(creds, statuses)
	if err != nil {
		return nil, fmt.Errorf("configure %s: %w", provider, err)
	}
	return p, nil
}
