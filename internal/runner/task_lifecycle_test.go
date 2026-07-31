package runner

import (
	"testing"

	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
)

type fakeTaskStore struct {
	tasks  []state.Task
	moved  string
	status string
}

func (f *fakeTaskStore) ListTasks() ([]state.Task, error) { return f.tasks, nil }

func (f *fakeTaskStore) MoveTask(id, status string, _ int) error {
	f.moved = id
	f.status = status
	return nil
}

func TestAdvanceTaskForCompletedImplementationMovesLinkedTaskToReview(t *testing.T) {
	store := &fakeTaskStore{tasks: []state.Task{
		{ID: "task-1", SessionID: "session-1", Status: task.StatusInProgress.String()},
		{ID: "task-2", SessionID: "other-session", Status: task.StatusInProgress.String()},
		{ID: "task-3", SessionID: "session-1", Status: task.StatusInReview.String()},
	}}

	r := &Runner{tasks: store}
	r.advanceTaskForCompletedImplementation("session-1")

	if store.moved != "task-1" {
		t.Fatalf("moved task = %q, want task-1", store.moved)
	}
	if store.status != task.StatusInReview.String() {
		t.Fatalf("moved status = %q, want %s", store.status, task.StatusInReview)
	}
}

func TestAdvanceTaskForCompletedImplementationIgnoresNonImplementationTasks(t *testing.T) {
	store := &fakeTaskStore{tasks: []state.Task{
		{ID: "task-1", SessionID: "session-1", Status: task.StatusInReview.String()},
	}}

	r := &Runner{tasks: store}
	r.advanceTaskForCompletedImplementation("session-1")

	if store.moved != "" {
		t.Fatalf("moved task = %q, want no move", store.moved)
	}
}
