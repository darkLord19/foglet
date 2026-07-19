// Package power provides keep-awake functionality to prevent idle sleep
// while AI agents are running.
package power

import (
	"os/exec"
	"sync"
)

// Inhibitor manages power management assertions with reference counting.
type Inhibitor struct {
	mu    sync.Mutex
	count int
	cmd   *exec.Cmd

	// startFn/stopFn override the platform assertion in tests. When nil the
	// real platform implementation is used.
	startFn func()
	stopFn  func()
}

// New creates a new Inhibitor.
func New() *Inhibitor {
	return &Inhibitor{}
}

// SetAssertionHooks overrides the platform keep-awake assertion with the given
// start/stop callbacks. It exists so tests can exercise ref-counting without
// spawning a real caffeinate child; production code leaves the hooks nil.
func (ih *Inhibitor) SetAssertionHooks(start, stop func()) {
	ih.mu.Lock()
	defer ih.mu.Unlock()
	ih.startFn = start
	ih.stopFn = stop
}

// Acquire increments the ref count and starts holding a power assertion
// if this is the first acquisition.
func (ih *Inhibitor) Acquire() {
	ih.mu.Lock()
	defer ih.mu.Unlock()
	ih.count++
	if ih.count == 1 {
		ih.start()
	}
}

// Release decrements the ref count and releases the power assertion
// when the count reaches zero.
func (ih *Inhibitor) Release() {
	ih.mu.Lock()
	defer ih.mu.Unlock()
	if ih.count <= 0 {
		return
	}
	ih.count--
	if ih.count == 0 {
		ih.stop()
	}
}

// Count returns the current ref count.
func (ih *Inhibitor) Count() int {
	ih.mu.Lock()
	defer ih.mu.Unlock()
	return ih.count
}

func (ih *Inhibitor) start() {
	if ih.startFn != nil {
		ih.startFn()
		return
	}
	ih.startPlatform()
}

func (ih *Inhibitor) stop() {
	if ih.stopFn != nil {
		ih.stopFn()
		return
	}
	ih.stopPlatform()
}
