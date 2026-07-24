// Package run defines the state machine for agent run lifecycle.
package run

import (
	"fmt"
	"strings"
)

// State is the lifecycle phase of one run.
type State string

const (
	StateCreated    State = "CREATED"
	StateQueued     State = "QUEUED"
	StateSetup      State = "SETUP"
	StateAIRunning  State = "AI_RUNNING"
	StateValidating State = "VALIDATING"
	StateCommitted  State = "COMMITTED"
	StateCompleted  State = "COMPLETED"
	StateFailed     State = "FAILED"
	StateCancelled  State = "CANCELLED"
)

// Terminal reports whether this state ends the run.
func (s State) Terminal() bool {
	switch s {
	case StateCompleted, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

// Valid reports whether this is a known state.
func (s State) Valid() bool {
	switch s {
	case StateCreated, StateQueued, StateSetup, StateAIRunning,
		StateValidating, StateCommitted, StateCompleted, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

func (s State) String() string { return string(s) }

// Parse normalises and validates a state string.
func Parse(raw string) (State, error) {
	s := State(strings.ToUpper(strings.TrimSpace(raw)))
	if !s.Valid() {
		return "", fmt.Errorf("unknown run state %q", raw)
	}
	return s, nil
}

// CanTransition reports whether a run may move from src to dst.
// Terminal states cannot transition; any non-terminal state may reach
// FAILED or CANCELLED; otherwise the transition must be strictly forward.
func CanTransition(src, dst State) bool {
	if src.Terminal() {
		return false
	}
	if dst == StateFailed || dst == StateCancelled {
		return true
	}
	order := map[State]int{
		StateCreated:    0,
		StateQueued:     1,
		StateSetup:      2,
		StateAIRunning:  3,
		StateValidating: 4,
		StateCommitted:  5,
		StateCompleted:  6,
	}
	srcOrd, okSrc := order[src]
	dstOrd, okDst := order[dst]
	return okSrc && okDst && dstOrd > srcOrd
}
