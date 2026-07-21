package tracker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
)

func TestStatusMapToFogMatchesLoosely(t *testing.T) {
	m := DefaultStatusMap()

	cases := map[string]task.Status{
		"Todo":         task.StatusTodo,
		"to do":        task.StatusTodo,
		"TO-DO":        task.StatusTodo,
		"Backlog":      task.StatusTodo,
		"In Progress":  task.StatusInProgress,
		"in-progress":  task.StatusInProgress,
		"IN  PROGRESS": task.StatusInProgress,
		"Code Review":  task.StatusInReview,
		"review":       task.StatusInReview,
		"Done":         task.StatusDone,
		"Merged":       task.StatusDone,
	}

	for remote, want := range cases {
		got, ok := m.ToFog(remote)
		if !ok {
			t.Errorf("ToFog(%q) not mapped", remote)
			continue
		}
		if got != want {
			t.Errorf("ToFog(%q) = %q, want %q", remote, got, want)
		}
	}
}

// An unrecognised upstream state must NOT fall back to Todo. Guessing wrong
// gets written back to the tracker on the next push, corrupting a board Fog
// does not own.
func TestStatusMapRefusesToGuess(t *testing.T) {
	m := DefaultStatusMap()

	for _, unknown := range []string{"Dev In Flight", "Peer Check", "Blocked", "", "   "} {
		if got, ok := m.ToFog(unknown); ok {
			t.Errorf("ToFog(%q) = %q; unknown states must not be mapped", unknown, got)
		}
	}
}

func TestStatusMapToRemoteUsesFirstEntry(t *testing.T) {
	m := DefaultStatusMap()

	got, ok := m.ToRemote(task.StatusInProgress)
	if !ok || got != "In Progress" {
		t.Errorf("ToRemote(in_progress) = (%q, %v), want (\"In Progress\", true)", got, ok)
	}

	if _, ok := m.ToRemote("nonsense"); ok {
		t.Error("ToRemote should reject an unknown Fog status")
	}
}

func TestStatusMapValidate(t *testing.T) {
	if err := DefaultStatusMap().Validate(); err != nil {
		t.Errorf("default map should be complete: %v", err)
	}

	partial := StatusMap{Todo: []string{"Open"}, Done: []string{"Closed"}}
	err := partial.Validate()
	if err == nil {
		t.Fatal("incomplete map should not validate")
	}
	// The error has to name what's missing, or the user can't fix it.
	for _, want := range []string{"in_progress", "in_review"} {
		if !contains(err.Error(), want) {
			t.Errorf("Validate error should mention %q, got: %v", want, err)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || len(needle) == 0 ||
			(len(haystack) > 0 && stringIndex(haystack, needle) >= 0))
}

func stringIndex(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// ── fakes ────────────────────────────────────────────────────────────────

type fakeProvider struct {
	issues []Issue
	pushed map[string]task.Status
	err    error
}

func newFakeProvider(issues ...Issue) *fakeProvider {
	return &fakeProvider{issues: issues, pushed: map[string]task.Status{}}
}

func (f *fakeProvider) Name() task.Provider { return task.ProviderLinear }

func (f *fakeProvider) List(context.Context) ([]Issue, error) {
	return f.issues, f.err
}

func (f *fakeProvider) SetStatus(_ context.Context, id string, s task.Status) error {
	f.pushed[id] = s
	return nil
}

type fakeStore struct {
	tasks []state.Task
	moves []string
}

func (s *fakeStore) ListTasks() ([]state.Task, error) { return s.tasks, nil }

func (s *fakeStore) CreateTask(t state.Task) (state.Task, error) {
	s.tasks = append(s.tasks, t)
	return t, nil
}

func (s *fakeStore) GetTaskByExternal(provider, externalID string) (state.Task, error) {
	for _, t := range s.tasks {
		if t.Provider == provider && t.ExternalID == externalID {
			return t, nil
		}
	}
	return state.Task{}, state.ErrTaskNotFound
}

func (s *fakeStore) MoveTask(id, status string, _ int) error {
	s.moves = append(s.moves, id+"->"+status)
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks[i].Status = status
		}
	}
	return nil
}

func (s *fakeStore) UpdateTask(t state.Task) error {
	for i := range s.tasks {
		if s.tasks[i].ID == t.ID {
			s.tasks[i] = t
		}
	}
	return nil
}

func (s *fakeStore) MarkTaskSynced(id, externalStatus string, _ time.Time) error {
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks[i].ExternalStatus = externalStatus
		}
	}
	return nil
}

// ── sync ─────────────────────────────────────────────────────────────────

func TestSyncImportsNewIssues(t *testing.T) {
	provider := newFakeProvider(
		Issue{ID: "i1", Key: "ENG-1", Title: "Add limiter", Status: "Todo"},
		Issue{ID: "i2", Key: "ENG-2", Title: "Fix test", Status: "In Progress"},
	)
	store := &fakeStore{}

	res, err := NewSyncer(provider, store, DefaultStatusMap()).Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Imported != 2 {
		t.Errorf("Imported = %d, want 2", res.Imported)
	}
	if len(store.tasks) != 2 {
		t.Fatalf("store has %d tasks, want 2", len(store.tasks))
	}
	if store.tasks[0].Status != "todo" || store.tasks[1].Status != "in_progress" {
		t.Errorf("statuses not mapped: %q, %q", store.tasks[0].Status, store.tasks[1].Status)
	}
	if store.tasks[0].Provider != string(task.ProviderLinear) {
		t.Errorf("Provider = %q", store.tasks[0].Provider)
	}
}

// The whole point of the origin rule: an issue arriving already In Progress
// gets mirrored into that column, but sync never launches an agent. It only
// touches the store.
func TestSyncNeverStartsWork(t *testing.T) {
	provider := newFakeProvider(
		Issue{ID: "i1", Key: "ENG-1", Title: "Remote start", Status: "In Progress"},
	)
	store := &fakeStore{}

	if _, err := NewSyncer(provider, store, DefaultStatusMap()).Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(store.tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(store.tasks))
	}
	got := store.tasks[0]
	if got.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", got.Status)
	}
	// No session was linked, because nothing was started.
	if got.SessionID != "" {
		t.Errorf("SessionID = %q; sync must never start an agent", got.SessionID)
	}
	// And the domain agrees this transition is not auto-startable remotely.
	if _, ok := task.AutoStarts(task.StatusTodo, task.StatusInProgress, task.OriginRemote); ok {
		t.Error("remote transition reported as auto-starting")
	}
}

func TestSyncSkipsUnmappedStatuses(t *testing.T) {
	provider := newFakeProvider(
		Issue{ID: "i1", Key: "ENG-1", Title: "Odd state", Status: "Dev In Flight"},
	)
	store := &fakeStore{}

	res, err := NewSyncer(provider, store, DefaultStatusMap()).Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Skipped != 1 || res.Imported != 0 {
		t.Errorf("Skipped=%d Imported=%d, want 1 and 0", res.Skipped, res.Imported)
	}
	if len(res.Unmapped) != 1 || res.Unmapped[0] != "Dev In Flight" {
		t.Errorf("Unmapped = %v, want [Dev In Flight]", res.Unmapped)
	}
	if len(store.tasks) != 0 {
		t.Error("an unmapped issue must not be imported")
	}
}

func TestSyncPushesLocalStatusChange(t *testing.T) {
	provider := newFakeProvider(
		Issue{ID: "i1", Key: "ENG-1", Title: "Moved here", Status: "Todo"},
	)
	store := &fakeStore{tasks: []state.Task{{
		ID: "t1", Title: "Moved here", Status: "done",
		Provider: string(task.ProviderLinear), ExternalID: "i1", ExternalStatus: "Todo",
	}}}

	res, err := NewSyncer(provider, store, DefaultStatusMap()).Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Pushed != 1 {
		t.Errorf("Pushed = %d, want 1", res.Pushed)
	}
	if provider.pushed["i1"] != task.StatusDone {
		t.Errorf("pushed %q, want done", provider.pushed["i1"])
	}
}

func TestSyncDoesNotPushWhenAlreadyInAgreement(t *testing.T) {
	provider := newFakeProvider(
		Issue{ID: "i1", Key: "ENG-1", Title: "Agreed", Status: "In Progress"},
	)
	store := &fakeStore{tasks: []state.Task{{
		ID: "t1", Title: "Agreed", Status: "in_progress",
		Provider: string(task.ProviderLinear), ExternalID: "i1", ExternalStatus: "In Progress",
	}}}

	res, err := NewSyncer(provider, store, DefaultStatusMap()).Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Pushed != 0 {
		t.Errorf("Pushed = %d, want 0", res.Pushed)
	}
	if len(provider.pushed) != 0 {
		t.Errorf("nothing should have been pushed, got %v", provider.pushed)
	}
}

// A task whose issue is no longer in the upstream result set is out of scope,
// not deleted. Pushing into the unknown would be worse than doing nothing.
func TestSyncIgnoresTasksMissingFromUpstream(t *testing.T) {
	provider := newFakeProvider()
	store := &fakeStore{tasks: []state.Task{{
		ID: "t1", Title: "Gone", Status: "done",
		Provider: string(task.ProviderLinear), ExternalID: "vanished", ExternalStatus: "Todo",
	}}}

	res, err := NewSyncer(provider, store, DefaultStatusMap()).Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Pushed != 0 || len(provider.pushed) != 0 {
		t.Error("must not push for an issue absent from upstream")
	}
}

func TestSyncPropagatesProviderFailure(t *testing.T) {
	provider := newFakeProvider()
	provider.err = errors.New("boom")

	if _, err := NewSyncer(provider, &fakeStore{}, DefaultStatusMap()).Sync(context.Background()); err == nil {
		t.Fatal("want error from provider")
	}
}

func TestProvidersRejectMissingCredentials(t *testing.T) {
	if _, err := NewLinear("", "", DefaultStatusMap()); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Linear: want ErrNotConfigured, got %v", err)
	}
	if _, err := NewJira("https://x.atlassian.net", "", "tok", "", DefaultStatusMap()); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Jira: want ErrNotConfigured, got %v", err)
	}
}

func TestFlattenADF(t *testing.T) {
	doc := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": "Rate limit "}},
			},
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": "the gateway."}},
			},
		},
	}
	if got := flattenADF(doc); got != "Rate limit \nthe gateway." {
		t.Errorf("flattenADF = %q", got)
	}
	if got := flattenADF("plain string"); got != "plain string" {
		t.Errorf("legacy string description: %q", got)
	}
	if got := flattenADF(nil); got != "" {
		t.Errorf("nil description: %q", got)
	}
}
