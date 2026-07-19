package ai

import (
	"reflect"
	"testing"
)

func TestBuildAntigravityHeadlessArgsWithStreamJSONAndAutoApprove(t *testing.T) {
	got := buildAntigravityHeadlessArgs(ExecuteRequest{
		Prompt:         "fix auth",
		Model:          "gemini-3-pro",
		ConversationID: "agy-session-1",
	}, true, true)
	want := []string{
		"--model", "gemini-3-pro",
		"--conversation", "agy-session-1",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
		"-p", "fix auth",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}

func TestBuildAntigravityHeadlessArgsWithoutStreamJSONOrAutoApprove(t *testing.T) {
	got := buildAntigravityHeadlessArgs(ExecuteRequest{Prompt: "fix auth"}, false, false)
	want := []string{"-p", "fix auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}
