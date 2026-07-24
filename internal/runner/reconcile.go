package runner

import (
	"fmt"

	runpkg "github.com/darkLord19/foglet/internal/run"
	"github.com/darkLord19/foglet/internal/state"
)

// ReconcileOnStart marks any non-terminal run that was active at daemon
// startup as FAILED. A run that is non-terminal at boot by definition
// belongs to a process that no longer exists.
func (r *Runner) ReconcileOnStart() error {
	if r.runs == nil {
		return nil
	}
	sessions, err := r.runs.ListSessions()
	if err != nil {
		return fmt.Errorf("reconcile: list sessions: %w", err)
	}
	for _, sess := range sessions {
		if !sess.Busy {
			continue
		}
		latest, found, err := r.runs.GetLatestRun(sess.ID)
		if err != nil {
			return fmt.Errorf("reconcile session %s: %w", sess.ID, err)
		}
		if !found {
			// Busy with no runs — just clear the flag.
			if err := r.runs.SetSessionBusy(sess.ID, false); err != nil {
				return fmt.Errorf("reconcile session %s: clear busy: %w", sess.ID, err)
			}
			continue
		}
		s := runpkg.State(latest.State)
		if s.Terminal() {
			// Run finished but flag wasn't cleared (e.g. crash after CompleteRun).
			if err := r.runs.SetSessionBusy(sess.ID, false); err != nil {
				return fmt.Errorf("reconcile session %s: clear busy: %w", sess.ID, err)
			}
			continue
		}
		const errMsg = "interrupted: daemon restarted"
		if err := r.runs.CompleteRun(latest.ID, runpkg.StateFailed.String(), "", "", errMsg); err != nil {
			return fmt.Errorf("reconcile run %s: complete: %w", latest.ID, err)
		}
		if err := r.runs.AppendRunEvent(state.RunEvent{
			RunID:   latest.ID,
			Type:    "interrupted",
			Message: errMsg,
		}); err != nil {
			return fmt.Errorf("reconcile run %s: event: %w", latest.ID, err)
		}
		if err := r.runs.UpdateSessionStatus(sess.ID, runpkg.StateFailed.String()); err != nil {
			return fmt.Errorf("reconcile session %s: status: %w", sess.ID, err)
		}
		if err := r.runs.SetSessionBusy(sess.ID, false); err != nil {
			return fmt.Errorf("reconcile session %s: clear busy: %w", sess.ID, err)
		}
	}
	return nil
}
