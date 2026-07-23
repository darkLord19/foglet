package state

import (
	"errors"
	"testing"
	"time"
)

func newTaskStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func mustCreateTask(t *testing.T, s *Store, id, title, status string) Task {
	t.Helper()
	task, err := s.CreateTask(Task{ID: id, Title: title, Status: status})
	if err != nil {
		t.Fatalf("CreateTask(%s): %v", id, err)
	}
	return task
}

// columnIDs returns the ids in a column, in board order.
func columnIDs(t *testing.T, s *Store, status string) []string {
	t.Helper()
	all, err := s.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	ids := []string{}
	for _, task := range all {
		if task.Status == status {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

func TestTrashRestoreAndPurge(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "t1", "keep", "todo")
	mustCreateTask(t, s, "t2", "toss", "done")

	// Trashing drops it from the board but not from the database.
	if err := s.TrashTask("t2"); err != nil {
		t.Fatalf("TrashTask: %v", err)
	}
	if ids := columnIDs(t, s, "done"); len(ids) != 0 {
		t.Errorf("done column after trash = %v, want empty", ids)
	}
	trashed, err := s.ListTrashedTasks()
	if err != nil {
		t.Fatalf("ListTrashedTasks: %v", err)
	}
	if len(trashed) != 1 || trashed[0].ID != "t2" || trashed[0].TrashedAt == nil {
		t.Fatalf("ListTrashedTasks = %+v, want t2 with TrashedAt set", trashed)
	}

	// A live task is still reachable by id; a trashed one still is too.
	if got, err := s.GetTask("t2"); err != nil || got.TrashedAt == nil {
		t.Fatalf("GetTask(t2) = %+v, err %v; want trashed", got, err)
	}

	// Restore returns it to its original column.
	if err := s.RestoreTask("t2"); err != nil {
		t.Fatalf("RestoreTask: %v", err)
	}
	if ids := columnIDs(t, s, "done"); len(ids) != 1 || ids[0] != "t2" {
		t.Errorf("done column after restore = %v, want [t2]", ids)
	}

	// Purge is a hard delete.
	if err := s.DeleteTask("t2"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := s.GetTask("t2"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("GetTask after delete: err %v, want ErrTaskNotFound", err)
	}
}

func TestListTrashedBefore(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "old", "old", "todo")
	mustCreateTask(t, s, "new", "new", "todo")
	if err := s.TrashTask("old"); err != nil {
		t.Fatalf("TrashTask(old): %v", err)
	}
	if err := s.TrashTask("new"); err != nil {
		t.Fatalf("TrashTask(new): %v", err)
	}

	// Backdate one trashed_at so it falls before the cutoff.
	if _, err := s.db.Exec(
		`UPDATE tasks SET trashed_at = ? WHERE id = ?`,
		"2000-01-01T00:00:00Z", "old",
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	// Cutoff sits between the backdated "old" and the just-trashed "new".
	cutoff := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	expired, err := s.ListTrashedBefore(cutoff)
	if err != nil {
		t.Fatalf("ListTrashedBefore: %v", err)
	}
	if len(expired) != 1 || expired[0].ID != "old" {
		t.Fatalf("ListTrashedBefore = %+v, want [old]", expired)
	}
}

func TestCreateTaskValidation(t *testing.T) {
	s := newTaskStore(t)

	cases := []struct {
		name string
		in   Task
	}{
		{"no id", Task{Title: "x", Status: "todo"}},
		{"no title", Task{ID: "t1", Status: "todo"}},
		{"blank title", Task{ID: "t1", Title: "   ", Status: "todo"}},
		{"no status", Task{ID: "t1", Title: "x"}},
	}

	for _, tc := range cases {
		if _, err := s.CreateTask(tc.in); err == nil {
			t.Errorf("CreateTask(%s): want error, got nil", tc.name)
		}
	}
}

func TestCreateTaskDefaultsProviderToLocal(t *testing.T) {
	s := newTaskStore(t)
	got := mustCreateTask(t, s, "t1", "Add rate limiting", "todo")
	if got.Provider != "local" {
		t.Errorf("Provider = %q, want local", got.Provider)
	}
	if got.Position == 0 {
		t.Error("Position should be assigned when not supplied")
	}
}

func TestCreateTaskAppendsToColumnEnd(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "a", "first", "todo")
	mustCreateTask(t, s, "b", "second", "todo")
	mustCreateTask(t, s, "c", "third", "todo")

	got := columnIDs(t, s, "todo")
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column order = %v, want %v", got, want)
		}
	}
}

func TestMoveTaskToIndex(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "a", "first", "todo")
	mustCreateTask(t, s, "b", "second", "todo")
	mustCreateTask(t, s, "c", "third", "todo")

	// Move the last card to the front of its own column.
	if err := s.MoveTask("c", "todo", 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if got := columnIDs(t, s, "todo"); got[0] != "c" {
		t.Errorf("after move to 0: %v, want c first", got)
	}

	// Move it into the middle.
	if err := s.MoveTask("c", "todo", 1); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	got := columnIDs(t, s, "todo")
	want := []string{"a", "c", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after move to 1: %v, want %v", got, want)
		}
	}
}

func TestMoveTaskAcrossColumns(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "a", "first", "todo")
	mustCreateTask(t, s, "b", "second", "todo")
	mustCreateTask(t, s, "x", "other", "in_progress")

	if err := s.MoveTask("a", "in_progress", 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	if got := columnIDs(t, s, "todo"); len(got) != 1 || got[0] != "b" {
		t.Errorf("todo column = %v, want [b]", got)
	}
	got := columnIDs(t, s, "in_progress")
	want := []string{"a", "x"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("in_progress column = %v, want %v", got, want)
		}
	}
}

// Repeatedly dropping a card into the same slot halves the gap each time.
// This guards the fractional scheme against collapsing to equal positions,
// which would make board order non-deterministic.
func TestMoveTaskRepeatedMidpointsStayDistinct(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "a", "a", "todo")
	mustCreateTask(t, s, "b", "b", "todo")
	mustCreateTask(t, s, "m", "mover", "todo")

	for i := 0; i < 30; i++ {
		if err := s.MoveTask("m", "todo", 1); err != nil {
			t.Fatalf("MoveTask iteration %d: %v", i, err)
		}
	}

	got := columnIDs(t, s, "todo")
	want := []string{"a", "m", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after 30 midpoint moves: %v, want %v", got, want)
		}
	}

	all, _ := s.ListTasks()
	seen := map[float64]string{}
	for _, task := range all {
		if prev, dup := seen[task.Position]; dup {
			t.Fatalf("position %v shared by %s and %s", task.Position, prev, task.ID)
		}
		seen[task.Position] = task.ID
	}
}

func TestMoveTaskClampsOutOfRangeIndex(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "a", "a", "todo")
	mustCreateTask(t, s, "b", "b", "todo")

	if err := s.MoveTask("a", "todo", 99); err != nil {
		t.Fatalf("MoveTask high index: %v", err)
	}
	if got := columnIDs(t, s, "todo"); got[len(got)-1] != "a" {
		t.Errorf("high index should land last, got %v", got)
	}

	if err := s.MoveTask("a", "todo", -5); err != nil {
		t.Fatalf("MoveTask negative index: %v", err)
	}
	if got := columnIDs(t, s, "todo"); got[0] != "a" {
		t.Errorf("negative index should land first, got %v", got)
	}
}

func TestGetUpdateDeleteTask(t *testing.T) {
	s := newTaskStore(t)

	// repo_name carries a foreign key, so the repo has to exist before a task
	// can point at it.
	if _, err := s.UpsertRepo(Repo{
		Name: "foglet", URL: "https://github.com/darkLord19/foglet",
		Host: "github.com", BarePath: "/tmp/bare", BaseWorktreePath: "/tmp/wt",
	}); err != nil {
		t.Fatalf("UpsertRepo: %v", err)
	}

	mustCreateTask(t, s, "t1", "Original", "todo")

	got, err := s.GetTask("t1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Title != "Original" {
		t.Errorf("Title = %q", got.Title)
	}

	got.Title = "Renamed"
	got.RepoName = "foglet"
	if err := s.UpdateTask(got); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	after, _ := s.GetTask("t1")
	if after.Title != "Renamed" || after.RepoName != "foglet" {
		t.Errorf("update did not persist: %+v", after)
	}

	if err := s.DeleteTask("t1"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := s.GetTask("t1"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("after delete: want ErrTaskNotFound, got %v", err)
	}
}

func TestMissingTaskOperationsReportNotFound(t *testing.T) {
	s := newTaskStore(t)

	if err := s.MoveTask("ghost", "todo", 0); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("MoveTask: want ErrTaskNotFound, got %v", err)
	}
	if err := s.DeleteTask("ghost"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("DeleteTask: want ErrTaskNotFound, got %v", err)
	}
	if err := s.LinkTaskSession("ghost", "s1"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("LinkTaskSession: want ErrTaskNotFound, got %v", err)
	}
	if _, err := s.GetTask("ghost"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("GetTask: want ErrTaskNotFound, got %v", err)
	}
}

func TestExternalTaskLookup(t *testing.T) {
	s := newTaskStore(t)
	_, err := s.CreateTask(Task{
		ID: "t1", Title: "Synced issue", Status: "todo",
		Provider: "linear", ExternalID: "iss_abc", ExternalKey: "ENG-421",
		ExternalURL: "https://linear.app/x/issue/ENG-421",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := s.GetTaskByExternal("linear", "iss_abc")
	if err != nil {
		t.Fatalf("GetTaskByExternal: %v", err)
	}
	if got.ID != "t1" || got.ExternalKey != "ENG-421" {
		t.Errorf("unexpected task: %+v", got)
	}

	if _, err := s.GetTaskByExternal("linear", "nope"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("want ErrTaskNotFound, got %v", err)
	}
}

// Two local tasks both have NULL external_id; the unique index is partial so
// that must not collide.
func TestLocalTasksDoNotCollideOnNullExternalID(t *testing.T) {
	s := newTaskStore(t)
	mustCreateTask(t, s, "a", "first", "todo")

	if _, err := s.CreateTask(Task{ID: "b", Title: "second", Status: "todo"}); err != nil {
		t.Fatalf("second local task should be allowed: %v", err)
	}
}

func TestDuplicateExternalIDRejected(t *testing.T) {
	s := newTaskStore(t)
	base := Task{Title: "Issue", Status: "todo", Provider: "linear", ExternalID: "iss_1"}

	base.ID = "a"
	if _, err := s.CreateTask(base); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	base.ID = "b"
	if _, err := s.CreateTask(base); err == nil {
		t.Error("duplicate (provider, external_id) should be rejected")
	}
}
