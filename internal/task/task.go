package task

import (
	"encoding/json"
	"fmt"
	"time"
)

// State represents the task lifecycle state
type State string

const (
	StateCreated    State = "CREATED"
	StateSetup      State = "SETUP"
	StateAIRunning  State = "AI_RUNNING"
	StateValidating State = "VALIDATING"
	StateCommitted  State = "COMMITTED"
	StatePRCreated  State = "PR_CREATED"
	StateCompleted  State = "COMPLETED"
	StateFailed     State = "FAILED"
	StateCancelled  State = "CANCELLED"
)

// Task represents an AI coding task
type Task struct {
	ID           string                 `json:"id"`
	State        State                  `json:"state"`
	Branch       string                 `json:"branch"`
	Prompt       string                 `json:"prompt"`
	AITool       string                 `json:"ai_tool"`
	WorktreePath string                 `json:"worktree_path"`
	Options      Options                `json:"options"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Options contains task execution options
type Options struct {
	Commit       bool   `json:"commit"`
	CreatePR     bool   `json:"create_pr"`
	Validate     bool   `json:"validate"`
	BaseBranch   string `json:"base_branch"`
	CommitMsg    string `json:"commit_msg,omitempty"`
	SetupCmd     string `json:"setup_cmd"`
	ValidateCmd  string `json:"validate_cmd"`
	Async        bool   `json:"async"`
	SlackChannel string `json:"slack_channel,omitempty"`
}

// StateTransition represents a state change
type StateTransition struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// CanTransitionTo checks if a state transition is valid
func (s State) CanTransitionTo(next State) bool {
	validTransitions := map[State][]State{
		StateCreated:    {StateSetup, StateFailed},
		StateSetup:      {StateAIRunning, StateFailed, StateCancelled},
		StateAIRunning:  {StateValidating, StateCommitted, StateFailed, StateCancelled},
		StateValidating: {StateCommitted, StateFailed, StateCancelled},
		StateCommitted:  {StatePRCreated, StateCompleted, StateFailed, StateCancelled},
		StatePRCreated:  {StateCompleted, StateFailed, StateCancelled},
		StateCompleted:  {},
		StateFailed:     {},
		StateCancelled:  {},
	}

	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}

	for _, a := range allowed {
		if a == next {
			return true
		}
	}

	return false
}

// TransitionTo changes the task state
func (t *Task) TransitionTo(newState State) error {
	if !t.State.CanTransitionTo(newState) {
		return fmt.Errorf("invalid transition from %s to %s", t.State, newState)
	}

	t.State = newState
	t.UpdatedAt = time.Now()

	if newState == StateCompleted || newState == StateFailed || newState == StateCancelled {
		now := time.Now()
		t.CompletedAt = &now
	}

	return nil
}

// SetError sets the task error and transitions to FAILED
func (t *Task) SetError(err error) {
	t.Error = err.Error()
	t.State = StateFailed
	t.UpdatedAt = time.Now()
	now := time.Now()
	t.CompletedAt = &now
}

// ToJSON converts task to JSON
func (t *Task) ToJSON() ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}

// FromJSON creates a task from JSON
func FromJSON(data []byte) (*Task, error) {
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// IsTerminal returns true if the task is in a terminal state
func (t *Task) IsTerminal() bool {
	return t.State == StateCompleted || t.State == StateFailed || t.State == StateCancelled
}

// Duration returns the task execution duration
func (t *Task) Duration() time.Duration {
	if t.CompletedAt != nil {
		return t.CompletedAt.Sub(t.CreatedAt)
	}
	return time.Since(t.CreatedAt)
}
