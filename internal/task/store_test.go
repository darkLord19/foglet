package task

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveGetListDelete(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	t1 := &Task{
		ID:        "task-1",
		State:     StateCreated,
		Branch:    "feature-a",
		Prompt:    "do thing",
		AITool:    "claude",
		CreatedAt: time.Now().Add(-2 * time.Minute),
		UpdatedAt: time.Now().Add(-2 * time.Minute),
	}
	t2 := &Task{
		ID:        "task-2",
		State:     StateCompleted,
		Branch:    "feature-b",
		Prompt:    "do thing 2",
		AITool:    "cursor",
		CreatedAt: time.Now().Add(-1 * time.Minute),
		UpdatedAt: time.Now().Add(-1 * time.Minute),
	}

	if err := store.Save(t1); err != nil {
		t.Fatalf("save t1 failed: %v", err)
	}
	if err := store.Save(t2); err != nil {
		t.Fatalf("save t2 failed: %v", err)
	}

	got, err := store.Get("task-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.ID != "task-1" || got.Branch != "feature-a" {
		t.Fatalf("unexpected task payload: %+v", got)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("unexpected list count: got %d want 2", len(all))
	}
	if all[0].ID != "task-2" {
		t.Fatalf("expected newest task first, got %s", all[0].ID)
	}

	active, err := store.ListActive()
	if err != nil {
		t.Fatalf("list active failed: %v", err)
	}
	if len(active) != 1 || active[0].ID != "task-1" {
		t.Fatalf("unexpected active tasks: %+v", active)
	}

	if err := store.Delete("task-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	remaining, err := store.List()
	if err != nil {
		t.Fatalf("list after delete failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "task-2" {
		t.Fatalf("unexpected remaining tasks: %+v", remaining)
	}
}

func TestStoreUsesSQLiteNotTaskFiles(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	task := &Task{
		ID:        "task-file-check",
		State:     StateCreated,
		Branch:    "feature-check",
		Prompt:    "check files",
		AITool:    "claude",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(tmp, "tasks", "*.json"))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no json task files, found: %v", matches)
	}
}
