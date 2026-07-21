package runner

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/power"
	"github.com/darkLord19/foglet/internal/state"
)

// fakeRunStore is an in-memory RunStore. It records everything the pipeline
// writes, in order, so tests can assert on observable outcomes at the seam
// rather than on internal runner state.
type fakeRunStore struct {
	mu sync.Mutex

	runs     map[string]*state.Run
	sessions map[string]*state.Session
	events   []state.RunEvent

	// runStates is the ordered sequence of phases passed to SetRunState, which
	// is the pipeline's phase transcript.
	runStates      []string
	sessionStates  []string
	busyWrites     []bool
	prURLs         []string
	latestRunID    string
	latestSession  string
	failOn         map[string]error
	appendedEvents int
}

func newFakeRunStore() *fakeRunStore {
	return &fakeRunStore{
		runs:     map[string]*state.Run{},
		sessions: map[string]*state.Session{},
		failOn:   map[string]error{},
	}
}

// seed registers a session and its latest run so GetLatestRun resolves.
func (f *fakeRunStore) seed(sessionID, runID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[sessionID] = &state.Session{ID: sessionID}
	f.runs[runID] = &state.Run{ID: runID, SessionID: sessionID}
	f.latestRunID = runID
	f.latestSession = sessionID
}

func (f *fakeRunStore) fail(method string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOn[method] = err
}

func (f *fakeRunStore) check(method string) error {
	if err, ok := f.failOn[method]; ok {
		return err
	}
	return nil
}

func (f *fakeRunStore) SetRunState(id, runState string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("SetRunState"); err != nil {
		return err
	}
	f.runStates = append(f.runStates, runState)
	if run, ok := f.runs[id]; ok {
		run.State = runState
	}
	return nil
}

func (f *fakeRunStore) CompleteRun(id, runState, commitSHA, commitMsg, runErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("CompleteRun"); err != nil {
		return err
	}
	f.runStates = append(f.runStates, runState)
	if run, ok := f.runs[id]; ok {
		run.State = runState
		run.CommitSHA = commitSHA
		run.CommitMsg = commitMsg
		run.Error = runErr
	}
	return nil
}

func (f *fakeRunStore) AppendRunEvent(event state.RunEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("AppendRunEvent"); err != nil {
		return err
	}
	f.events = append(f.events, event)
	f.appendedEvents++
	return nil
}

func (f *fakeRunStore) ListRuns(sessionID string) ([]state.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("ListRuns"); err != nil {
		return nil, err
	}
	var out []state.Run
	for _, run := range f.runs {
		if run.SessionID == sessionID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (f *fakeRunStore) ListRunEvents(runID string, limit int) ([]state.RunEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("ListRunEvents"); err != nil {
		return nil, err
	}
	var out []state.RunEvent
	for _, e := range f.events {
		if e.RunID == runID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRunStore) GetLatestRun(sessionID string) (state.Run, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("GetLatestRun"); err != nil {
		return state.Run{}, false, err
	}
	if sessionID != f.latestSession || f.latestRunID == "" {
		return state.Run{}, false, nil
	}
	return *f.runs[f.latestRunID], true, nil
}

func (f *fakeRunStore) UpdateSessionStatus(id, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("UpdateSessionStatus"); err != nil {
		return err
	}
	f.sessionStates = append(f.sessionStates, status)
	return nil
}

func (f *fakeRunStore) SetSessionBusy(id string, busy bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("SetSessionBusy"); err != nil {
		return err
	}
	f.busyWrites = append(f.busyWrites, busy)
	return nil
}

func (f *fakeRunStore) SetSessionPRURL(id, prURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check("SetSessionPRURL"); err != nil {
		return err
	}
	f.prURLs = append(f.prURLs, prURL)
	return nil
}

// eventTypes returns the ordered list of appended event types.
func (f *fakeRunStore) eventTypes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.events))
	for _, e := range f.events {
		out = append(out, e.Type)
	}
	return out
}

// eventOfType returns the first event of the given type.
func (f *fakeRunStore) eventOfType(t string) (state.RunEvent, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.events {
		if e.Type == t {
			return e, true
		}
	}
	return state.RunEvent{}, false
}

// busyCleared reports whether the session was released.
func (f *fakeRunStore) busyCleared() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.busyWrites {
		if !b {
			return true
		}
	}
	return false
}

// fakeSettings is an in-memory SettingsReader.
type fakeSettings map[string]string

func (f fakeSettings) GetSetting(key string) (string, bool, error) {
	v, ok := f[key]
	return v, ok, nil
}

// fakeTool is an ai.Tool that never spawns a process.
type fakeTool struct {
	name           string
	available      bool
	output         string
	conversationID string
	err            error
	chunks         []string

	// gotRequest captures what the pipeline asked for, so tests can assert the
	// prompt, model and resumed conversation id crossing the seam.
	mu         sync.Mutex
	gotRequest ai.ExecuteRequest
	calls      int

	// block, when non-nil, is waited on before returning — used to test
	// cancellation mid-run.
	block func(ctx context.Context) error
}

func (f *fakeTool) Name() string      { return f.name }
func (f *fakeTool) IsAvailable() bool { return f.available }

func (f *fakeTool) Execute(ctx context.Context, workdir, prompt string) (*ai.Result, error) {
	return f.ExecuteStream(ctx, ai.ExecuteRequest{Workdir: workdir, Prompt: prompt}, nil)
}

func (f *fakeTool) ExecuteStream(ctx context.Context, req ai.ExecuteRequest, onChunk func(string)) (*ai.Result, error) {
	f.mu.Lock()
	f.gotRequest = req
	f.calls++
	f.mu.Unlock()

	for _, c := range f.chunks {
		if onChunk != nil {
			onChunk(c)
		}
	}

	if f.block != nil {
		if err := f.block(ctx); err != nil {
			return &ai.Result{Success: false, Error: err}, err
		}
	}

	if f.err != nil {
		return &ai.Result{Success: false, Output: f.output, Error: f.err}, f.err
	}
	return &ai.Result{
		Success:        true,
		Output:         f.output,
		ConversationID: f.conversationID,
	}, nil
}

func (f *fakeTool) request() ai.ExecuteRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotRequest
}

// toolFactory returns a ToolFactory serving the given tool for any name.
func toolFactory(tool *fakeTool) ToolFactory {
	return func(name string) (ai.Tool, error) {
		if tool == nil {
			return nil, fmt.Errorf("unknown AI tool: %s", name)
		}
		return tool, nil
	}
}

// newSilentInhibitor returns a power.Inhibitor whose assertion is a no-op, so
// tests never spawn a real caffeinate child.
func newSilentInhibitor() *power.Inhibitor {
	ih := power.New()
	ih.SetAssertionHooks(func() {}, func() {})
	return ih
}

// newTestRunner builds a Runner wired entirely to fakes.
func newTestRunner(store *fakeRunStore, tool *fakeTool, settings fakeSettings) *Runner {
	if settings == nil {
		settings = fakeSettings{}
	}
	return &Runner{
		runs:     store,
		settings: settings,
		tools:    toolFactory(tool),
		baseCtx:  context.Background(),
		power:    newSilentInhibitor(),
		active:   map[string]*activeRun{},
	}
}
