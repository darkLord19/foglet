package power

import (
	"sync"
	"testing"
)

// newTestInhibitor returns an inhibitor whose start/stop are counted instead of
// spawning a real platform assertion (e.g. caffeinate).
func newTestInhibitor() (*Inhibitor, *int, *int) {
	var starts, stops int
	ih := New()
	ih.startFn = func() { starts++ }
	ih.stopFn = func() { stops++ }
	return ih, &starts, &stops
}

func TestInhibitorRefCounting(t *testing.T) {
	ih, starts, stops := newTestInhibitor()

	ih.Acquire() // 0 -> 1: start
	if *starts != 1 || *stops != 0 || ih.Count() != 1 {
		t.Fatalf("after first acquire: starts=%d stops=%d count=%d", *starts, *stops, ih.Count())
	}

	ih.Acquire() // 1 -> 2: no additional start
	if *starts != 1 || ih.Count() != 2 {
		t.Fatalf("after second acquire: starts=%d count=%d", *starts, ih.Count())
	}

	ih.Release() // 2 -> 1: no stop yet
	if *stops != 0 || ih.Count() != 1 {
		t.Fatalf("after first release: stops=%d count=%d", *stops, ih.Count())
	}

	ih.Release() // 1 -> 0: stop
	if *stops != 1 || ih.Count() != 0 {
		t.Fatalf("after last release: stops=%d count=%d", *stops, ih.Count())
	}
}

// TestInhibitorReleaseWithoutAcquire guards against a negative ref count, which
// would desync the assertion from the number of active runs.
func TestInhibitorReleaseWithoutAcquire(t *testing.T) {
	ih, starts, stops := newTestInhibitor()

	ih.Release()
	if ih.Count() != 0 || *starts != 0 || *stops != 0 {
		t.Fatalf("release without acquire: count=%d starts=%d stops=%d", ih.Count(), *starts, *stops)
	}

	// A subsequent acquire must still start cleanly.
	ih.Acquire()
	if ih.Count() != 1 || *starts != 1 {
		t.Fatalf("acquire after stray release: count=%d starts=%d", ih.Count(), *starts)
	}
}

func TestInhibitorConcurrentAcquireRelease(t *testing.T) {
	ih, starts, stops := newTestInhibitor()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ih.Acquire()
			ih.Release()
		}()
	}
	wg.Wait()

	if ih.Count() != 0 {
		t.Fatalf("expected count 0 after balanced concurrent ops, got %d", ih.Count())
	}
	if *starts != *stops {
		t.Fatalf("start/stop imbalance: starts=%d stops=%d", *starts, *stops)
	}
}
