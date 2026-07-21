package slack

import (
	"strings"
	"testing"
)

func TestParseCommandTextAllOptions(t *testing.T) {
	cmd, err := parseCommandText("@fog [repo='acme-api' tool='cursor' model='gpt-5' autopr=true branch-name='feat/login' commit-msg='add login flow'] implement login")
	if err != nil {
		t.Fatalf("parseCommandText failed: %v", err)
	}

	if cmd.Repo != "acme-api" || cmd.Tool != "cursor" || cmd.Model != "gpt-5" {
		t.Fatalf("unexpected parsed command: %+v", cmd)
	}
	if !cmd.AutoPR || cmd.BranchName != "feat/login" || cmd.CommitMsg != "add login flow" {
		t.Fatalf("unexpected options: %+v", cmd)
	}
	if cmd.Prompt != "implement login" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParseCommandTextRepoRequired(t *testing.T) {
	_, err := parseCommandText("@fog [tool='claude'] do something")
	if err == nil || !strings.Contains(err.Error(), "repo is required") {
		t.Fatalf("expected repo required error, got: %v", err)
	}
}

func TestParseCommandTextUnknownOption(t *testing.T) {
	_, err := parseCommandText("@fog [repo='acme-api' foo='bar'] prompt")
	if err == nil || !strings.Contains(err.Error(), "unknown option key") {
		t.Fatalf("expected unknown option key error, got: %v", err)
	}
}

func TestParseCommandTextInvalidAutoPR(t *testing.T) {
	_, err := parseCommandText("@fog [repo='acme-api' autopr=yes] prompt")
	if err == nil || !strings.Contains(err.Error(), "invalid autopr value") {
		t.Fatalf("expected invalid autopr error, got: %v", err)
	}
}
