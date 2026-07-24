package runner

import (
	"testing"

	runpkg "github.com/darkLord19/foglet/internal/run"
	"github.com/darkLord19/foglet/internal/state"
)

func TestReconcileOnStartClearsOrphanedRuns(t *testing.T) {
	store := newFakeRunStore()

	sessionID := "sess-orphan"
	runID := "run-orphan"
	store.sessions[sessionID] = &state.Session{ID: sessionID, Busy: true}
	store.runs[runID] = &state.Run{
		ID:        runID,
		SessionID: sessionID,
		State:     runpkg.StateAIRunning.String(),
	}
	store.latestRunID = runID
	store.latestSession = sessionID

	r := newTestRunner(store, nil, nil)
	if err := r.ReconcileOnStart(); err != nil {
		t.Fatalf("ReconcileOnStart: %v", err)
	}

	run, ok := store.runs[runID]
	if !ok {
		t.Fatal("run disappeared")
	}
	if !runpkg.State(run.State).Terminal() {
		t.Fatalf("expected terminal state, got %q", run.State)
	}
	if run.State != runpkg.StateFailed.String() {
		t.Fatalf("expected FAILED, got %q", run.State)
	}
	if !store.busyCleared() {
		t.Fatal("expected session busy to be cleared")
	}
	var found bool
	for _, e := range store.events {
		if e.Type == "interrupted" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected interrupted event")
	}
}

func TestReconcileOnStartSkipsTerminalRuns(t *testing.T) {
	store := newFakeRunStore()

	sessionID := "sess-term"
	runID := "run-term"
	store.sessions[sessionID] = &state.Session{ID: sessionID, Busy: true}
	store.runs[runID] = &state.Run{
		ID:        runID,
		SessionID: sessionID,
		State:     runpkg.StateCompleted.String(),
	}
	store.latestRunID = runID
	store.latestSession = sessionID

	r := newTestRunner(store, nil, nil)
	if err := r.ReconcileOnStart(); err != nil {
		t.Fatalf("ReconcileOnStart: %v", err)
	}

	if !store.busyCleared() {
		t.Fatal("expected busy to be cleared")
	}
	for _, e := range store.events {
		if e.Type == "interrupted" {
			t.Fatal("did not expect interrupted event for terminal run")
		}
	}
}
