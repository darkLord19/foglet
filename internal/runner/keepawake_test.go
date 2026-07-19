package runner

import (
	"context"
	"testing"

	"github.com/darkLord19/foglet/internal/state"
)

// newCountingRunner returns a runner backed by a real state store whose power
// inhibitor counts acquire/release instead of spawning caffeinate.
func newCountingRunner(t *testing.T) (*Runner, *int, *int) {
	t.Helper()
	st, err := state.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	r, err := New(t.TempDir(), t.TempDir(), st)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var starts, stops int
	r.power.SetAssertionHooks(func() { starts++ }, func() { stops++ })
	return r, &starts, &stops
}

// TestKeepAwakeReleasedWhenToggledOffMidRun is the regression test for the
// ref-count leak: a run that acquired the assertion must release it on clear
// even if keep_awake is switched off while the run is in flight.
func TestKeepAwakeReleasedWhenToggledOffMidRun(t *testing.T) {
	r, starts, stops := newCountingRunner(t)

	if err := r.state.SetSetting("keep_awake", "true"); err != nil {
		t.Fatalf("SetSetting on: %v", err)
	}

	r.registerActiveRun("sess-1", "run-1", context.CancelFunc(func() {}))
	if *starts != 1 || r.power.Count() != 1 {
		t.Fatalf("after register with keep_awake on: starts=%d count=%d", *starts, r.power.Count())
	}

	// Toggle the setting off while the run is still active.
	if err := r.state.SetSetting("keep_awake", "false"); err != nil {
		t.Fatalf("SetSetting off: %v", err)
	}

	r.clearActiveRun("sess-1", "run-1")
	if *stops != 1 {
		t.Fatalf("expected release despite mid-run toggle-off, stops=%d", *stops)
	}
	if r.power.Count() != 0 {
		t.Fatalf("keep-awake assertion leaked: count=%d", r.power.Count())
	}
}

// TestKeepAwakeNotAcquiredWhenSettingOff verifies the assertion is untouched
// for runs that start with keep_awake disabled, even if it is toggled on later.
func TestKeepAwakeNotAcquiredWhenSettingOff(t *testing.T) {
	r, starts, stops := newCountingRunner(t)

	// keep_awake defaults to off (unset).
	r.registerActiveRun("sess-1", "run-1", context.CancelFunc(func() {}))
	if *starts != 0 || r.power.Count() != 0 {
		t.Fatalf("run started with keep_awake off should not acquire: starts=%d count=%d", *starts, r.power.Count())
	}

	// Toggle on mid-run; the already-registered run must not release on clear.
	if err := r.state.SetSetting("keep_awake", "true"); err != nil {
		t.Fatalf("SetSetting on: %v", err)
	}
	r.clearActiveRun("sess-1", "run-1")
	if *stops != 0 || r.power.Count() != 0 {
		t.Fatalf("run that never acquired must not release: stops=%d count=%d", *stops, r.power.Count())
	}
}
