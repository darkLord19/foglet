package runner

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"

	runpkg "github.com/darkLord19/foglet/internal/run"
	"github.com/darkLord19/foglet/internal/state"
)

// scheduledJob is one queued or running unit of work.
type scheduledJob struct {
	sessionID string
	repoName  string
	runID     string
	fn        func()

	mu        sync.Mutex
	cancelled bool
}

func (j *scheduledJob) markCancelled() {
	j.mu.Lock()
	j.cancelled = true
	j.mu.Unlock()
}

func (j *scheduledJob) isCancelled() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cancelled
}

// scheduler is a bounded worker pool with a FIFO queue and per-repo
// concurrency limits.
type scheduler struct {
	maxGlobal  int
	maxPerRepo int

	mu      sync.Mutex
	queue   []*scheduledJob
	running int
	perRepo map[string]int

	notify chan struct{}
	store  RunStore // for state transitions (nil in tests)
}

func defaultMaxConcurrent() int {
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	return n
}

func newScheduler(ctx context.Context, store RunStore, maxGlobal, maxPerRepo int) *scheduler {
	s := &scheduler{
		maxGlobal:  maxGlobal,
		maxPerRepo: maxPerRepo,
		perRepo:    make(map[string]int),
		notify:     make(chan struct{}, 1),
		store:      store,
	}
	go s.dispatch(ctx)
	return s
}

// Submit adds a job to the queue, sets the run state to QUEUED, appends
// a queued event with position, and signals the dispatcher. Non-blocking.
func (s *scheduler) Submit(job *scheduledJob) {
	s.mu.Lock()
	s.queue = append(s.queue, job)
	pos := len(s.queue)
	s.mu.Unlock()

	if s.store != nil {
		_ = s.store.SetRunState(job.runID, runpkg.StateQueued.String())
		data, _ := json.Marshal(map[string]int{"position": pos})
		_ = s.store.AppendRunEvent(state.RunEvent{
			RunID: job.runID,
			Type:  "queued",
			Data:  string(data),
		})
	}
	s.wake()
}

// Dequeue removes the queued job for sessionID before it starts.
// Returns (job, true) if found.
func (s *scheduler) Dequeue(sessionID string) (*scheduledJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, j := range s.queue {
		if j.sessionID == sessionID {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return j, true
		}
	}
	return nil, false
}

func (s *scheduler) wake() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *scheduler) dispatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.notify:
			s.runReady()
		}
	}
}

func (s *scheduler) runReady() {
	for {
		s.mu.Lock()
		job := s.nextReady()
		if job == nil {
			s.mu.Unlock()
			return
		}
		s.running++
		s.perRepo[job.repoName]++
		s.mu.Unlock()

		go func(j *scheduledJob) {
			if !j.isCancelled() {
				j.fn()
			}
			s.mu.Lock()
			s.running--
			s.perRepo[j.repoName]--
			if s.perRepo[j.repoName] == 0 {
				delete(s.perRepo, j.repoName)
			}
			s.mu.Unlock()
			s.wake()
		}(job)
	}
}

// nextReady returns the first queued job that fits both global and per-repo
// limits, after compacting cancelled entries. Must be called with s.mu held.
func (s *scheduler) nextReady() *scheduledJob {
	if s.running >= s.maxGlobal {
		return nil
	}
	// Compact cancelled jobs in-place.
	live := s.queue[:0]
	for _, j := range s.queue {
		if !j.isCancelled() {
			live = append(live, j)
		}
	}
	s.queue = live

	for i, j := range s.queue {
		if s.perRepo[j.repoName] < s.maxPerRepo {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return j
		}
	}
	return nil
}

// Drain cancels all queued jobs and waits for running jobs to finish,
// or until ctx expires.
func (s *scheduler) Drain(ctx context.Context) {
	s.mu.Lock()
	for _, j := range s.queue {
		j.markCancelled()
		if s.store != nil {
			_ = s.store.CompleteRun(j.runID, runpkg.StateCancelled.String(), "", "", "daemon shutting down")
			_ = s.store.SetSessionBusy(j.sessionID, false)
		}
	}
	s.queue = nil
	s.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.notify:
			s.mu.Lock()
			done := s.running == 0
			s.mu.Unlock()
			if done {
				return
			}
		}
	}
}
