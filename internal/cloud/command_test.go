package cloud

import (
	"strings"
	"testing"
)

func TestParseCommandText_AllOptions(t *testing.T) {
	cmd, err := parseCommandText("@fog [repo='acme/api' tool='cursor' model='gpt-5' autopr=true branch-name='feat/login' commit-msg='add login'] implement login")
	if err != nil {
		t.Fatalf("parseCommandText failed: %v", err)
	}
	if cmd.Repo != "acme/api" || cmd.Tool != "cursor" || cmd.Model != "gpt-5" {
		t.Fatalf("unexpected parsed command: %+v", cmd)
	}
	if !cmd.AutoPR || cmd.BranchName != "feat/login" || cmd.CommitMsg != "add login" {
		t.Fatalf("unexpected parsed options: %+v", cmd)
	}
	if cmd.Prompt != "implement login" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParseCommandText_RepoRequired(t *testing.T) {
	_, err := parseCommandText("@fog [tool='claude'] prompt")
	if err == nil || !strings.Contains(err.Error(), "repo is required") {
		t.Fatalf("expected repo required error, got: %v", err)
	}
}

func TestNormalizeFollowUpPrompt(t *testing.T) {
	if _, err := normalizeFollowUpPrompt(" "); err == nil {
		t.Fatal("expected empty prompt error")
	}
	if _, err := normalizeFollowUpPrompt("[repo='x'] prompt"); err == nil {
		t.Fatal("expected options rejection")
	}
	got, err := normalizeFollowUpPrompt("refine edge cases")
	if err != nil {
		t.Fatalf("unexpected normalizeFollowUpPrompt error: %v", err)
	}
	if got != "refine edge cases" {
		t.Fatalf("unexpected normalized prompt: %q", got)
	}
}
