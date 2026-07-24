package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerMaxConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const limit = 2
	const total = 5

	sched := newScheduler(ctx, nil, limit, 10)

	var active atomic.Int32
	var maxSeen atomic.Int32
	var wg sync.WaitGroup
	release := make(chan struct{})

	for i := 0; i < total; i++ {
		wg.Add(1)
		idx := i
		job := &scheduledJob{
			sessionID: fmt.Sprintf("s%d", idx),
			repoName:  fmt.Sprintf("repo%d", idx), // unique repos avoid per-repo limit
			runID:     fmt.Sprintf("r%d", idx),
			fn: func() {
				defer wg.Done()
				n := active.Add(1)
				for {
					cur := maxSeen.Load()
					if n <= cur || maxSeen.CompareAndSwap(cur, n) {
						break
					}
				}
				<-release
				active.Add(-1)
			},
		}
		sched.Submit(job)
	}

	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := maxSeen.Load(); got > limit {
		t.Fatalf("max concurrent = %d, want <= %d", got, limit)
	}
}

func TestSchedulerPerRepoSerialization(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sched := newScheduler(ctx, nil, 10, 1) // per-repo limit 1, global limit high

	var active atomic.Int32
	var maxSeen atomic.Int32
	var wg sync.WaitGroup
	release := make(chan struct{})

	const total = 3
	for i := 0; i < total; i++ {
		wg.Add(1)
		idx := i
		job := &scheduledJob{
			sessionID: fmt.Sprintf("s%d", idx),
			repoName:  "same-repo",
			runID:     fmt.Sprintf("r%d", idx),
			fn: func() {
				defer wg.Done()
				n := active.Add(1)
				for {
					cur := maxSeen.Load()
					if n <= cur || maxSeen.CompareAndSwap(cur, n) {
						break
					}
				}
				<-release
				active.Add(-1)
			},
		}
		sched.Submit(job)
	}

	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := maxSeen.Load(); got > 1 {
		t.Fatalf("max concurrent for same repo = %d, want 1", got)
	}
}

func TestSchedulerCancelQueuedJobNeverExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sched := newScheduler(ctx, nil, 1, 1) // global limit 1

	release := make(chan struct{})
	started := make(chan struct{})

	job1 := &scheduledJob{
		sessionID: "s1",
		repoName:  "repo",
		runID:     "r1",
		fn: func() {
			close(started)
			<-release
		},
	}
	sched.Submit(job1)
	<-started // job1 is now running and holds the slot

	var job2Ran bool
	job2 := &scheduledJob{
		sessionID: "s2",
		repoName:  "repo",
		runID:     "r2",
		fn: func() {
			job2Ran = true
		},
	}
	sched.Submit(job2)

	// Dequeue and cancel job2 while job1 holds the slot.
	dequeued, ok := sched.Dequeue("s2")
	if !ok {
		t.Fatal("expected job2 to be in queue")
	}
	dequeued.markCancelled()

	close(release)
	time.Sleep(50 * time.Millisecond)

	if job2Ran {
		t.Fatal("cancelled job2 should not have run")
	}
}
