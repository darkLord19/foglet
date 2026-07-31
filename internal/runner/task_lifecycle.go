package runner

import (
	"log"

	"github.com/darkLord19/foglet/internal/task"
)

const appendTaskPosition = 1 << 30

// advanceTaskForCompletedImplementation keeps the board in sync with the
// session outcome. The task API remains the only place that starts agents;
// this method only advances an implementation task into review, so a run
// completion cannot accidentally launch a second process during persistence.
func (r *Runner) advanceTaskForCompletedImplementation(sessionID string) {
	if r == nil || r.tasks == nil || sessionID == "" {
		return
	}

	tasks, err := r.tasks.ListTasks()
	if err != nil {
		log.Printf("runner: list tasks after implementation %s: %v", sessionID, err)
		return
	}

	for _, candidate := range tasks {
		if candidate.SessionID != sessionID || candidate.Status != task.StatusInProgress.String() {
			continue
		}
		if err := r.tasks.MoveTask(candidate.ID, task.StatusInReview.String(), appendTaskPosition); err != nil {
			log.Printf("runner: advance task %s to review: %v", candidate.ID, err)
		}
	}
}
