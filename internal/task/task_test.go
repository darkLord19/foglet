package task

import (
	"errors"
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	cases := []struct {
		in      string
		want    Status
		wantErr bool
	}{
		{"todo", StatusTodo, false},
		{"  TODO  ", StatusTodo, false},
		{"in_progress", StatusInProgress, false},
		{"In_Review", StatusInReview, false},
		{"done", StatusDone, false},
		{"", "", true},
		{"backlog", "", true},
		{"in progress", "", true},
	}

	for _, tc := range cases {
		got, err := ParseStatus(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseStatus(%q): want error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseStatus(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanTransition(t *testing.T) {
	// The board is permissive by design: any real move between known columns is
	// allowed, including backwards.
	for _, from := range Statuses() {
		for _, to := range Statuses() {
			err := CanTransition(from, to)
			switch {
			case from == to && err == nil:
				t.Errorf("CanTransition(%s -> %s): want no-op rejected", from, to)
			case from != to && err != nil:
				t.Errorf("CanTransition(%s -> %s): unexpected error: %v", from, to, err)
			}
		}
	}

	if err := CanTransition("todo", "shipped"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("unknown target: want ErrInvalidTransition, got %v", err)
	}
	if err := CanTransition("archived", "todo"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("unknown source: want ErrInvalidTransition, got %v", err)
	}
}

// TestAutoStartsRequiresLocalOrigin is the security-critical case: a status
// change arriving from a shared external tracker must never launch an agent on
// this machine, into either working column.
func TestAutoStartsRequiresLocalOrigin(t *testing.T) {
	for _, to := range []Status{StatusInProgress, StatusInReview} {
		for _, from := range Statuses() {
			if from == to {
				continue
			}
			for _, origin := range []Origin{OriginRemote, OriginSystem} {
				if _, ok := AutoStarts(from, to, origin); ok {
					t.Errorf("AutoStarts(%s -> %s, %s) started work; "+
						"non-local origins must never start an agent", from, to, origin)
				}
			}
		}
	}
}

func TestAutoStartsKindPerColumn(t *testing.T) {
	cases := []struct {
		from, to Status
		wantKind WorkKind
		wantOK   bool
	}{
		{StatusTodo, StatusInProgress, WorkImplement, true},
		{StatusDone, StatusInProgress, WorkImplement, true},
		{StatusInProgress, StatusInReview, WorkReview, true},
		{StatusTodo, StatusInReview, WorkReview, true},
		{StatusInReview, StatusDone, WorkNone, false},
		{StatusInProgress, StatusTodo, WorkNone, false},
		{StatusInReview, StatusTodo, WorkNone, false},
	}

	for _, tc := range cases {
		kind, ok := AutoStarts(tc.from, tc.to, OriginLocal)
		if ok != tc.wantOK || kind != tc.wantKind {
			t.Errorf("AutoStarts(%s -> %s, local) = (%q, %v), want (%q, %v)",
				tc.from, tc.to, kind, ok, tc.wantKind, tc.wantOK)
		}
	}
}

func TestAutoStartsIgnoresReentry(t *testing.T) {
	// Re-entering a column must not spawn a second run of the same kind.
	for _, s := range []Status{StatusInProgress, StatusInReview} {
		if _, ok := AutoStarts(s, s, OriginLocal); ok {
			t.Errorf("AutoStarts(%s -> %s) started work, want none", s, s)
		}
	}
}

func TestReviewPromptIsReadOnlyAndCarriesTheTask(t *testing.T) {
	got := ReviewPrompt("Add rate limiting", "100 req/min per key")

	for _, want := range []string{"Add rate limiting", "100 req/min per key", "Do not modify files"} {
		if !strings.Contains(got, want) {
			t.Errorf("ReviewPrompt missing %q", want)
		}
	}

	// A body is optional; the prompt must still be coherent without one.
	if bare := ReviewPrompt("Fix flaky test", ""); !strings.Contains(bare, "Fix flaky test") {
		t.Error("ReviewPrompt dropped the title when body was empty")
	}
}

func TestStatusForRunState(t *testing.T) {
	cases := []struct {
		state    string
		want     Status
		wantMove bool
	}{
		{"COMPLETED", StatusInReview, true},
		{"completed", StatusInReview, true},
		{"  COMPLETED ", StatusInReview, true},
		{"FAILED", "", false},
		{"CANCELLED", "", false},
		{"AI_RUNNING", "", false},
		{"", "", false},
	}

	for _, tc := range cases {
		got, move := StatusForRunState(tc.state)
		if move != tc.wantMove {
			t.Errorf("StatusForRunState(%q) move = %v, want %v", tc.state, move, tc.wantMove)
			continue
		}
		if move && got != tc.want {
			t.Errorf("StatusForRunState(%q) = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestValidators(t *testing.T) {
	if !Status("todo").Valid() || Status("nope").Valid() {
		t.Error("Status.Valid is wrong")
	}
	if !Origin("local").Valid() || Origin("nope").Valid() {
		t.Error("Origin.Valid is wrong")
	}
	if !Provider("linear").Valid() || Provider("nope").Valid() {
		t.Error("Provider.Valid is wrong")
	}
	if len(Statuses()) != 4 {
		t.Errorf("Statuses() returned %d columns, want 4", len(Statuses()))
	}
}
