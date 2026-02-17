package ai

import (
	"reflect"
	"testing"
)

func TestBuildGeminiHeadlessArgsWithStreamJSONAndYolo(t *testing.T) {
	got := buildGeminiHeadlessArgs(ExecuteRequest{
		Prompt:         "fix auth",
		Model:          "gemini-2.0-pro",
		ConversationID: "gemini-session-1",
	}, true, true)
	want := []string{
		"--model", "gemini-2.0-pro",
		"--resume", "gemini-session-1",
		"--output-format", "stream-json",
		"--yolo",
		"-p", "fix auth",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}

func TestBuildGeminiHeadlessArgsWithoutStreamJSONOrYolo(t *testing.T) {
	got := buildGeminiHeadlessArgs(ExecuteRequest{Prompt: "fix auth"}, false, false)
	want := []string{"-p", "fix auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}

