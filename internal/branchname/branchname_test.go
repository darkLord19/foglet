package branchname

import (
	"strings"
	"testing"
)

// neverExists is the predicate for a repository with no branches.
func neverExists(string) bool { return false }

// existing returns a predicate reporting membership of the given names.
func existing(names ...string) func(string) bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(candidate string) bool { return set[candidate] }
}

func TestResolveSlugifiesPromptUnderPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		prompt string
		want   string
	}{
		{"lowercases and hyphenates", "team", "Add OTP Login", "team/add-otp-login"},
		{"strips punctuation", "team", "Add OTP Login!!", "team/add-otp-login"},
		{"collapses runs of separators", "team", "fix   the __ bug", "team/fix-the-bug"},
		{"trims leading and trailing separators", "team", "  !!wrap!!  ", "team/wrap"},
		{"defaults the prefix", "", "Add login", DefaultPrefix + "/add-login"},
		{"strips slashes from the prefix", "/team/", "Add login", "team/add-login"},
		{"drops non-ascii", "team", "café ☕ time", "team/caf-time"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve("", tc.prefix, tc.prompt, neverExists)
			if err != nil {
				t.Fatalf("Resolve returned error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Resolve() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveReturnsRequestedNameUnchanged(t *testing.T) {
	// An explicit name is the caller's choice — not slugified, and not made
	// unique, even when it collides.
	got, err := Resolve("feature/login", "team", "some prompt", existing("feature/login"))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "feature/login" {
		t.Errorf("Resolve() = %q, want %q", got, "feature/login")
	}
}

func TestResolveRejectsInvalidRequestedName(t *testing.T) {
	for _, requested := range []string{
		"bad branch name",
		"feature//bad",
		"feature/..bad",
		"feature/bad@{1}",
		"/leading",
		"trailing/",
		"has~tilde",
		"has:colon",
		"",
	} {
		t.Run(requested, func(t *testing.T) {
			if _, err := Resolve(requested, "team", "", neverExists); err == nil {
				// An empty requested name falls through to slug generation, so it
				// is the one input above that must succeed.
				if requested == "" {
					return
				}
				t.Fatalf("Resolve(%q) succeeded, want error", requested)
			}
		})
	}
}

func TestResolveAppendsSuffixOnCollision(t *testing.T) {
	got, err := Resolve("", "fog", "Task Collision", existing("fog/task-collision"))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "fog/task-collision-1" {
		t.Errorf("Resolve() = %q, want %q", got, "fog/task-collision-1")
	}
}

func TestResolveWalksPastConsecutiveCollisions(t *testing.T) {
	got, err := Resolve("", "fog", "Task Collision", existing(
		"fog/task-collision",
		"fog/task-collision-1",
		"fog/task-collision-2",
	))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "fog/task-collision-3" {
		t.Errorf("Resolve() = %q, want %q", got, "fog/task-collision-3")
	}
}

func TestResolveFallsBackWhenEverySuffixTaken(t *testing.T) {
	// Exhaust the numeric search entirely; the timestamp fallback must still
	// produce a legal, non-colliding name rather than looping or erroring.
	got, err := Resolve("", "fog", "Task Collision", func(string) bool { return true })
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !strings.HasPrefix(got, "fog/task-collision-") {
		t.Errorf("Resolve() = %q, want the fog/task-collision- prefix", got)
	}
	if got == "fog/task-collision-100" {
		t.Error("Resolve() returned a numeric suffix, want the timestamp fallback")
	}
	if _, err := Validate(got); err != nil {
		t.Errorf("fallback produced an invalid name %q: %v", got, err)
	}
}

func TestResolveWithoutExistsSkipsUniqueness(t *testing.T) {
	// A nil predicate means the caller cannot reach a repository. The name must
	// still be legal.
	got, err := Resolve("", "fog", "Task Collision", nil)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "fog/task-collision" {
		t.Errorf("Resolve() = %q, want %q", got, "fog/task-collision")
	}
}

func TestResolveNeverExceedsMaxLen(t *testing.T) {
	longPrompt := strings.Repeat("very long prompt segment ", 40)

	t.Run("no collision", func(t *testing.T) {
		got, err := Resolve("", "team", longPrompt, neverExists)
		if err != nil {
			t.Fatalf("Resolve returned error: %v", err)
		}
		if len(got) > MaxLen {
			t.Errorf("len(Resolve()) = %d, want <= %d", len(got), MaxLen)
		}
	})

	t.Run("with numeric suffix", func(t *testing.T) {
		// The truncation must make room for the suffix, not append past MaxLen.
		var seen []string
		got, err := Resolve("", "team", longPrompt, func(c string) bool {
			seen = append(seen, c)
			return len(seen) == 1 // collide once, forcing a "-1" suffix
		})
		if err != nil {
			t.Fatalf("Resolve returned error: %v", err)
		}
		if len(got) > MaxLen {
			t.Errorf("len(Resolve()) = %d, want <= %d", len(got), MaxLen)
		}
		if !strings.HasSuffix(got, "-1") {
			t.Errorf("Resolve() = %q, want a -1 suffix", got)
		}
	})

	t.Run("with timestamp fallback", func(t *testing.T) {
		got, err := Resolve("", "team", longPrompt, func(string) bool { return true })
		if err != nil {
			t.Fatalf("Resolve returned error: %v", err)
		}
		if len(got) > MaxLen {
			t.Errorf("len(Resolve()) = %d, want <= %d", len(got), MaxLen)
		}
	})
}

func TestResolveGeneratesNameForEmptyPrompt(t *testing.T) {
	got, err := Resolve("", "fog", "   ", neverExists)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !strings.HasPrefix(got, "fog/task-") {
		t.Errorf("Resolve() = %q, want a fog/task- timestamp name", got)
	}
	if _, err := Validate(got); err != nil {
		t.Errorf("generated name %q is invalid: %v", got, err)
	}
}

func TestResolvePassesOnlyLegalCandidatesToExists(t *testing.T) {
	// The predicate is handed real branch names, so a caller may use it to hit
	// git directly without sanitising first.
	Resolve("", "fog", "Add OTP Login", func(candidate string) bool {
		if _, err := Validate(candidate); err != nil {
			t.Errorf("exists received invalid candidate %q: %v", candidate, err)
		}
		return false
	})
}

func TestValidate(t *testing.T) {
	valid := []string{"main", "fog/add-login", "team/fix-1", strings.Repeat("a", MaxLen)}
	for _, v := range valid {
		if _, err := Validate(v); err != nil {
			t.Errorf("Validate(%q) returned error: %v", truncateForMsg(v), err)
		}
	}

	invalid := map[string]string{
		"":                          "empty",
		"   ":                       "whitespace only",
		strings.Repeat("a", MaxLen+1): "too long",
		"/leading":                  "leading slash",
		"trailing/":                 "trailing slash",
		"a..b":                      "double dot",
		"a//b":                      "double slash",
		"a@{b":                      "reflog syntax",
		"a b":                       "space",
		"a~b":                       "tilde",
		"a^b":                       "caret",
		"a:b":                       "colon",
		"a?b":                       "question mark",
		"a*b":                       "asterisk",
		"a[b":                       "bracket",
		"a\\b":                      "backslash",
	}
	for value, reason := range invalid {
		if _, err := Validate(value); err == nil {
			t.Errorf("Validate(%q) succeeded, want error (%s)", truncateForMsg(value), reason)
		}
	}
}

func TestValidateTrimsSurroundingWhitespace(t *testing.T) {
	got, err := Validate("  fog/add-login  ")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if got != "fog/add-login" {
		t.Errorf("Validate() = %q, want %q", got, "fog/add-login")
	}
}

func truncateForMsg(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}
