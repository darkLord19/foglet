package runner

import (
	"strings"
	"sync"
	"time"

	"github.com/darkLord19/foglet/internal/state"
)

type runStreamWriter struct {
	mu        sync.Mutex
	store     state.RunEventSink
	runID     string
	buffer    strings.Builder
	lastFlush time.Time
}

func newRunStreamWriter(store state.RunEventSink, runID string) *runStreamWriter {
	return &runStreamWriter{
		store:     store,
		runID:     runID,
		lastFlush: time.Now().UTC(),
	}
}

func (w *runStreamWriter) Append(chunk string) {
	if chunk == "" {
		return
	}

	w.mu.Lock()
	w.buffer.WriteString(chunk)
	shouldFlush := w.buffer.Len() >= 1000 || time.Since(w.lastFlush) >= 600*time.Millisecond
	w.mu.Unlock()
	if shouldFlush {
		w.Flush()
	}
}

func (w *runStreamWriter) Flush() {
	if w == nil || w.store == nil {
		return
	}

	w.mu.Lock()
	payload := w.buffer.String()
	w.buffer.Reset()
	if strings.TrimSpace(payload) != "" {
		w.lastFlush = time.Now().UTC()
	}
	w.mu.Unlock()
	if strings.TrimSpace(payload) == "" {
		return
	}

	_ = w.store.AppendRunEvent(state.RunEvent{
		RunID: w.runID,
		Type:  "ai_stream",
		Data:  truncate(payload, 8000),
	})
}
