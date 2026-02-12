package ai

import (
	"reflect"
	"testing"
)

func TestBuildCursorHeadlessArgsWithOutputFormat(t *testing.T) {
	got := buildCursorHeadlessArgs("fix auth", true)
	want := []string{"-p", "--force", "--output-format", "text", "fix auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}

func TestBuildCursorHeadlessArgsWithoutOutputFormat(t *testing.T) {
	got := buildCursorHeadlessArgs("fix auth", false)
	want := []string{"-p", "--force", "fix auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %v want %v", got, want)
	}
}
