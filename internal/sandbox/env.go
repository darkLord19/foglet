package sandbox

import "strings"

// baseEnvAllowlist holds variables every agent process needs regardless of which
// tool is running: shell and locale basics, plus the proxy and CA settings that
// corporate networks depend on. Dropping the latter would break agents behind a
// TLS-inspecting proxy, which is a common enough setup that it is worth keeping
// even though it widens the surface slightly.
//
// Names are matched case-insensitively so the lowercase proxy spellings
// (http_proxy) are covered by a single entry.
var baseEnvAllowlist = map[string]struct{}{
	"HOME": {}, "PATH": {}, "USER": {}, "LOGNAME": {}, "SHELL": {},
	"TMPDIR": {}, "TZ": {},

	"TERM": {}, "COLORTERM": {}, "TERM_PROGRAM": {}, "TERM_PROGRAM_VERSION": {},
	"LANG": {}, "LANGUAGE": {},

	"HTTP_PROXY": {}, "HTTPS_PROXY": {}, "FTP_PROXY": {}, "ALL_PROXY": {},
	"NO_PROXY": {},

	"SSL_CERT_FILE": {}, "SSL_CERT_DIR": {}, "CURL_CA_BUNDLE": {},
	"REQUESTS_CA_BUNDLE": {}, "NODE_EXTRA_CA_CERTS": {},

	// The supported agent CLIs are Node programs, so their runtime lookup vars
	// have to survive. Listed exactly rather than by NODE_ prefix, because that
	// prefix would also admit NODE_AUTH_TOKEN — an npm registry credential.
	"NODE_OPTIONS": {}, "NODE_PATH": {}, "NVM_DIR": {}, "NVM_BIN": {},
}

// baseEnvPrefixes are prefix matches applied on top of baseEnvAllowlist. Both
// cover configuration rather than credentials.
var baseEnvPrefixes = []string{"LC_", "XDG_"}

// FilterEnv returns the subset of env an agent process should inherit.
//
// The daemon's environment can hold credentials for services the agent has no
// business touching — cloud keys, database URLs, registry tokens. This is an
// allowlist rather than a denylist so that an unanticipated secret is excluded
// by default instead of leaking until someone thinks to name it.
//
// toolPrefixes admits the running tool's own configuration (for example
// ANTHROPIC_ for Claude Code), which is the one credential family the agent
// legitimately needs. Scoping it per tool keeps one agent's key out of a
// different agent's environment.
//
// Returns nil when the guard is disabled, which callers pass straight through
// to mean "inherit the parent environment unchanged".
func FilterEnv(env []string, toolPrefixes []string) []string {
	if Disabled() {
		return nil
	}

	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if allowedEnvName(name, toolPrefixes) {
			out = append(out, entry)
		}
	}
	return out
}

func allowedEnvName(name string, toolPrefixes []string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if upper == "" {
		return false
	}
	if _, ok := baseEnvAllowlist[upper]; ok {
		return true
	}
	for _, prefix := range baseEnvPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	for _, prefix := range toolPrefixes {
		prefix = strings.ToUpper(strings.TrimSpace(prefix))
		if prefix != "" && strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}
