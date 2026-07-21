// Package task defines the work-item domain that sits above sessions.
//
// A Session only exists once an agent is running. A Task exists before that:
// it is the unit a person creates, prioritises, and drags across a board. One
// Task owns at most one Session at a time — starting work links them, and the
// Session's outcome drives the Task forward.
//
// This package holds the domain rules only (statuses, transitions, and the
// origin gate that decides when a move is allowed to launch an agent).
// Persistence lives in internal/state; transport lives in internal/api.
package task

import (
	"errors"
	"fmt"
	"strings"
)

// Status is a column on the board.
//
// These four are Fog's canonical vocabulary. External trackers have their own
// (and Jira's are user-configurable per project), so a provider mapping
// translates at the boundary rather than leaking foreign statuses inward.
type Status string

const (
	// StatusTodo is queued work. No session exists yet.
	StatusTodo Status = "todo"
	// StatusInProgress means an implementation agent is running, or has been
	// asked to.
	StatusInProgress Status = "in_progress"
	// StatusInReview means a reviewer agent is critiquing the implementation
	// in the session's own worktree. Review is local: no pull request, no
	// remote round trip.
	StatusInReview Status = "in_review"
	// StatusDone is terminal — the human accepted the work.
	StatusDone Status = "done"
)

// WorkKind is which agent a transition starts.
type WorkKind string

const (
	// WorkNone means the transition starts nothing.
	WorkNone WorkKind = ""
	// WorkImplement runs the implementation agent in a fresh session.
	WorkImplement WorkKind = "implement"
	// WorkReview runs a reviewer agent as a follow-up run on the existing
	// session, so it reads the same worktree the implementation produced.
	WorkReview WorkKind = "review"
)

// Statuses returns the canonical board columns in display order.
func Statuses() []Status {
	return []Status{StatusTodo, StatusInProgress, StatusInReview, StatusDone}
}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusInReview, StatusDone:
		return true
	default:
		return false
	}
}

func (s Status) String() string { return string(s) }

// ParseStatus normalises and validates a status string.
func ParseStatus(raw string) (Status, error) {
	s := Status(strings.ToLower(strings.TrimSpace(raw)))
	if !s.Valid() {
		return "", fmt.Errorf("unknown task status %q", raw)
	}
	return s, nil
}

// Origin records where a status change came from.
//
// This is a security boundary, not bookkeeping. Fog auto-starts an agent when a
// card is dragged into In Progress — which is only safe when the drag happened
// in Fog's own UI, on this machine. A status change arriving from a shared
// tracker was performed by someone else, possibly on another continent, and
// must never launch a process here. See AutoStarts.
type Origin string

const (
	// OriginLocal is a change made in Fog's own UI by the person at the machine.
	OriginLocal Origin = "local"
	// OriginRemote is a change observed on an external tracker during sync.
	OriginRemote Origin = "remote"
	// OriginSystem is a change Fog made itself, e.g. a run completing.
	OriginSystem Origin = "system"
)

// Valid reports whether o is a known origin.
func (o Origin) Valid() bool {
	switch o {
	case OriginLocal, OriginRemote, OriginSystem:
		return true
	default:
		return false
	}
}

// Provider identifies where a task's canonical record lives.
type Provider string

const (
	// ProviderLocal means Fog owns the task outright.
	ProviderLocal Provider = "local"
	// ProviderLinear mirrors a Linear issue.
	ProviderLinear Provider = "linear"
	// ProviderJira mirrors a Jira issue.
	ProviderJira Provider = "jira"
)

// Valid reports whether p is a known provider.
func (p Provider) Valid() bool {
	switch p {
	case ProviderLocal, ProviderLinear, ProviderJira:
		return true
	default:
		return false
	}
}

// ErrInvalidTransition is returned when a move is not permitted.
var ErrInvalidTransition = errors.New("invalid task transition")

// CanTransition reports whether a task may move from one status to another.
//
// The board is deliberately permissive: people reprioritise, and a card that
// can't go back to Todo is a worse tool than one that can. The only rejected
// move is a no-op, which callers should treat as "nothing happened" rather
// than as an error worth surfacing.
func CanTransition(from, to Status) error {
	if !from.Valid() {
		return fmt.Errorf("%w: unknown source status %q", ErrInvalidTransition, from)
	}
	if !to.Valid() {
		return fmt.Errorf("%w: unknown target status %q", ErrInvalidTransition, to)
	}
	if from == to {
		return fmt.Errorf("%w: already %s", ErrInvalidTransition, to)
	}
	return nil
}

// AutoStarts reports which agent, if any, a transition should launch
// immediately and without asking.
//
// Two columns start work:
//
//   - In Progress runs the implementation agent in a new session.
//   - In Review runs a reviewer agent as a follow-up on that same session, so
//     it reads the worktree the implementation just wrote.
//
// Three conditions gate it:
//
//  1. The target column is one that starts work.
//  2. The task was not already in that column — re-entering is a no-op, not a
//     second run.
//  3. The change originated locally.
//
// Condition 3 is the important one. Fog and an external tracker are
// bidirectionally synced, so a teammate dragging a card in Linear or Jira
// produces exactly the same status change as the owner dragging it here. If
// origin were ignored, anyone with write access to the tracker could execute an
// autonomous agent on this machine. Remote moves therefore reclassify the card
// and surface a Start affordance; the human at the keyboard decides.
func AutoStarts(from, to Status, origin Origin) (WorkKind, bool) {
	if from == to || origin != OriginLocal {
		return WorkNone, false
	}

	switch to {
	case StatusInProgress:
		return WorkImplement, true
	case StatusInReview:
		return WorkReview, true
	default:
		return WorkNone, false
	}
}

// StatusForRunState maps a terminal agent run state onto a board status, and
// reports whether the task should move at all.
//
// A completed implementation run does not mean the work is done — it means
// there is something to review, so the card advances to In Review and the
// reviewer agent picks it up. Nothing auto-advances to Done: accepting the work
// is a human judgement. Failures stay put, because dragging a failed card back
// yourself is clearer than Fog silently undoing your intent.
func StatusForRunState(runState string) (Status, bool) {
	switch strings.ToUpper(strings.TrimSpace(runState)) {
	case "COMPLETED":
		return StatusInReview, true
	case "FAILED", "CANCELLED":
		return "", false
	default:
		return "", false
	}
}

// ReviewPrompt is the instruction handed to the reviewer agent.
//
// It is deliberately read-only: a reviewer that edits code is just a second
// implementer, and the human loses the independent read they moved the card to
// get. The original task travels with it so the review is against intent rather
// than against the diff alone.
func ReviewPrompt(title, body string) string {
	var b strings.Builder
	b.WriteString("You are reviewing an implementation, not writing one.\n\n")
	b.WriteString("Original task\n")
	b.WriteString("-------------\n")
	b.WriteString(strings.TrimSpace(title))
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		b.WriteString("\n\n")
		b.WriteString(trimmed)
	}
	b.WriteString("\n\nReview the changes in this worktree against that task. Report:\n")
	b.WriteString("  1. Requirements from the task that are missing or only partly done.\n")
	b.WriteString("  2. Correctness bugs, with the specific input or state that triggers them.\n")
	b.WriteString("  3. Tests that are missing for the behaviour that changed.\n")
	b.WriteString("  4. Anything risky a reader would want flagged.\n\n")
	b.WriteString("Do not modify files. If the work looks correct, say so plainly ")
	b.WriteString("rather than inventing problems to report.")
	return b.String()
}
