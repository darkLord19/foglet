package slack

import (
	"strings"
	"testing"
	"time"

	"github.com/darkLord19/foglet/internal/state"
)

func TestStripMentions(t *testing.T) {
	got := stripMentions("<@U123> [repo='acme-api'] add auth")
	if got != "[repo='acme-api'] add auth" {
		t.Fatalf("unexpected stripped text: %q", got)
	}
}

func TestNormalizeFollowUpPrompt(t *testing.T) {
	if _, err := normalizeFollowUpPrompt("   "); err == nil {
		t.Fatalf("expected empty prompt error")
	}
	if _, err := normalizeFollowUpPrompt("[repo='x'] do thing"); err == nil {
		t.Fatalf("expected options rejection for follow-up prompt")
	}
	got, err := normalizeFollowUpPrompt("fix error handling")
	if err != nil {
		t.Fatalf("expected valid follow-up prompt, got error: %v", err)
	}
	if got != "fix error handling" {
		t.Fatalf("unexpected prompt normalization: %q", got)
	}
}

func TestCompletionTextFromSession(t *testing.T) {
	start := time.Now().Add(-4 * time.Second)
	end := time.Now()
	session := &state.Session{
		Branch: "fog/auth",
		PRURL:  "https://github.com/acme/repo/pull/1",
	}
	run := &state.Run{
		State:       "COMPLETED",
		CreatedAt:   start,
		CompletedAt: &end,
	}
	ok := completionTextFromSession(session, run)
	if !strings.Contains(ok, "Session completed") || !strings.Contains(ok, "PR: https://github.com/acme/repo/pull/1") {
		t.Fatalf("unexpected completion text: %s", ok)
	}

	failRun := &state.Run{
		State: "FAILED",
		Error: "boom",
	}
	fail := completionTextFromSession(session, failRun)
	if !strings.Contains(fail, "Session FAILED") {
		t.Fatalf("unexpected failure text: %s", fail)
	}
}
