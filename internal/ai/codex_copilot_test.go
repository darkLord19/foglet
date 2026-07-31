package ai

import (
	"reflect"
	"testing"
)

func TestBuildCodexArgs(t *testing.T) {
	got := buildCodexArgs(ExecuteRequest{Prompt: "fix auth", Model: "gpt-5", ConversationID: "session-1"}, true)
	want := []string{"exec", "resume", "session-1", "--json", "--model", "gpt-5", "fix auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}

func TestBuildCopilotArgs(t *testing.T) {
	got := buildCopilotArgs(ExecuteRequest{Prompt: "fix auth", Model: "gpt-5"}, true)
	want := []string{"-p", "--allow-all-tools", "--model", "gpt-5", "--output-format", "json", "fix auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}

func TestToolAliasesAndRegistrations(t *testing.T) {
	for _, name := range []string{"codex", "openai-codex", "copilot", "github-copilot"} {
		if _, err := GetTool(name); err != nil {
			t.Fatalf("GetTool(%q): %v", name, err)
		}
	}
	if len(AvailableToolNames()) != 5 {
		t.Fatalf("available tools = %v, want five tools", AvailableToolNames())
	}
}
