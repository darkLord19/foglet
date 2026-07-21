// Package branchname owns the rules for naming a session branch.
//
// Fog derives a branch name from the prompt a person typed, under a configurable
// prefix, and must guarantee the result is both a legal git ref and unique in the
// target repository. Those rules previously existed twice — once in the runner and
// once in the cloud relay — and the copies had already drifted: the relay's omitted
// the uniqueness check entirely, so it could hand back a name the runner would have
// rejected.
//
// The module is pure. Uniqueness is decided by an `exists` predicate supplied by the
// caller, so the rules can be tested without a git repository, and the prefix is a
// parameter rather than a settings lookup.
package branchname

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MaxLen is git's limit on a ref name, in bytes.
const MaxLen = 255

// DefaultPrefix is used when a caller supplies no prefix.
const DefaultPrefix = "fog"

// maxCollisionAttempts bounds the numeric-suffix search before falling back to a
// timestamp suffix. A repository with 100 colliding names is pathological; the
// fallback guarantees termination without an unbounded loop.
const maxCollisionAttempts = 100

var nonSlugChar = regexp.MustCompile(`[^a-z0-9]+`)

// Resolve returns a valid, unique branch name.
//
// If requested is non-empty it is validated and returned unchanged — an explicit
// name is the caller's choice, and Fog does not silently rewrite it, including for
// uniqueness. Otherwise a slug is derived from prompt under prefix, and a numeric
// suffix is appended until exists reports the name is free.
//
// prefix defaults to DefaultPrefix when empty.
//
// exists reports whether a branch already exists in the target repository. A nil
// exists disables the uniqueness check — callers that cannot reach a repository get
// a legal name that may collide, which is strictly better than failing, but every
// caller that can supply the check should.
func Resolve(requested, prefix, prompt string, exists func(string) bool) (string, error) {
	if requested = strings.TrimSpace(requested); requested != "" {
		return Validate(requested)
	}

	if prefix = strings.TrimSpace(prefix); prefix == "" {
		prefix = DefaultPrefix
	}

	base := strings.Trim(prefix, "/") + "/" + slugify(prompt)
	if len(base) > MaxLen {
		base = strings.Trim(base[:MaxLen], "/.-")
	}

	if exists == nil {
		return Validate(base)
	}

	for i := 0; i <= maxCollisionAttempts; i++ {
		candidate := base
		if i > 0 {
			candidate = withSuffix(base, fmt.Sprintf("-%d", i))
		}
		if !exists(candidate) {
			return Validate(candidate)
		}
	}

	// Every numeric suffix was taken. Fall back to a timestamp, which is short
	// enough to stay inside MaxLen after truncation.
	return Validate(withSuffix(base, "-"+strconv.FormatInt(time.Now().UnixNano(), 36)))
}

// Validate reports whether value is a legal git branch name, returning it trimmed.
//
// This is deliberately stricter than git itself: Fog interpolates branch names into
// shell-free argv but also into worktree paths and PR bodies, so characters that are
// merely awkward are rejected rather than escaped.
func Validate(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return "", fmt.Errorf("branch name cannot be empty")
	case len(value) > MaxLen:
		return "", fmt.Errorf("branch name exceeds %d characters", MaxLen)
	case strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/"):
		return "", fmt.Errorf("branch name cannot start or end with '/'")
	case strings.Contains(value, ".."), strings.Contains(value, "//"), strings.Contains(value, "@{"):
		return "", fmt.Errorf("branch name contains invalid sequence")
	case strings.ContainsAny(value, " ~^:?*[\\"):
		return "", fmt.Errorf("branch name contains invalid character")
	}
	return value, nil
}

// slugify reduces a prompt to lowercase alphanumeric words joined by hyphens.
// An empty result falls back to a timestamp so the branch is never bare.
func slugify(prompt string) string {
	slug := nonSlugChar.ReplaceAllString(strings.ToLower(strings.TrimSpace(prompt)), "-")
	if slug = strings.Trim(slug, "-"); slug == "" {
		return "task-" + time.Now().UTC().Format("20060102150405")
	}
	return slug
}

// withSuffix appends suffix to base, truncating base so the result fits in MaxLen.
func withSuffix(base, suffix string) string {
	if maxBase := max(MaxLen-len(suffix), 1); len(base) > maxBase {
		base = strings.Trim(base[:maxBase], "/.-")
	}
	return base + suffix
}

// IsProtected reports whether name is a branch Fog refuses to run an agent on.
//
// These are the conventional integration branches. The rule exists for launches
// that did not originate at this machine — a Slack command, say — where writing
// straight to the trunk is never the intent.
func IsProtected(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "main", "master", "develop", "trunk":
		return true
	default:
		return false
	}
}
