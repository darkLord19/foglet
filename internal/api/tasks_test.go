package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/state"
)

func postJSON(t *testing.T, srv *Server, h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func createTaskViaAPI(t *testing.T, srv *Server, req CreateTaskRequest) state.Task {
	t.Helper()
	w := postJSON(t, srv, srv.handleTasks, "/api/tasks", req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create task: status %d body=%s", w.Code, w.Body.String())
	}
	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return resp.Task
}

func TestCreateTaskRequiresTitle(t *testing.T) {
	srv := newTestServer(t)

	for _, body := range []CreateTaskRequest{{}, {Title: "   "}} {
		w := postJSON(t, srv, srv.handleTasks, "/api/tasks", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("blank title: status %d, want 400", w.Code)
		}
	}
}

func TestCreateTaskDefaultsToTodo(t *testing.T) {
	srv := newTestServer(t)
	got := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "Add rate limiting"})

	if got.Status != "todo" {
		t.Errorf("Status = %q, want todo", got.Status)
	}
	if got.Provider != "local" {
		t.Errorf("Provider = %q, want local", got.Provider)
	}
	if got.ID == "" {
		t.Error("ID should be assigned")
	}
}

func TestCreateTaskRejectsUnknownStatusAndRepo(t *testing.T) {
	srv := newTestServer(t)

	w := postJSON(t, srv, srv.handleTasks, "/api/tasks",
		CreateTaskRequest{Title: "x", Status: "backlog"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown status: got %d, want 400", w.Code)
	}

	w = postJSON(t, srv, srv.handleTasks, "/api/tasks",
		CreateTaskRequest{Title: "x", Repo: "nope/missing"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown repo: got %d, want 400", w.Code)
	}
}

func TestListTasksReturnsBoardOrder(t *testing.T) {
	srv := newTestServer(t)
	createTaskViaAPI(t, srv, CreateTaskRequest{Title: "first"})
	createTaskViaAPI(t, srv, CreateTaskRequest{Title: "second"})

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	w := httptest.NewRecorder()
	srv.handleTasks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list: status %d body=%s", w.Code, w.Body.String())
	}
	var tasks []state.Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].Title != "first" {
		t.Errorf("order = %q first, want 'first'", tasks[0].Title)
	}
}

// A task with no repo cannot start work, so dragging it into In Progress must
// report the failure rather than silently landing the card as if it had run.
func TestMoveToInProgressWithoutRepoReportsStartFailure(t *testing.T) {
	srv := newTestServer(t)
	created := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "No repo set"})

	w := postJSON(t, srv, srv.handleTaskDetail, "/api/tasks/"+created.ID+"/move",
		MoveTaskRequest{Status: "in_progress", Index: 0})

	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409; body=%s", w.Code, w.Body.String())
	}

	// The move itself is kept: the card shows where the user put it.
	after, err := srv.stateStore.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if after.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (move is preserved)", after.Status)
	}
	if after.SessionID != "" {
		t.Errorf("SessionID = %q, want empty (nothing started)", after.SessionID)
	}
}

// Todo and Done start nothing, so they succeed regardless of whether the task
// is runnable. In Progress and In Review both start agents and are covered
// separately.
func TestMoveBetweenNonStartingColumns(t *testing.T) {
	srv := newTestServer(t)
	created := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "Parked"})

	for _, status := range []string{"done", "todo"} {
		w := postJSON(t, srv, srv.handleTaskDetail, "/api/tasks/"+created.ID+"/move",
			MoveTaskRequest{Status: status, Index: 0})
		if w.Code != http.StatusOK {
			t.Fatalf("move to %s: status %d body=%s", status, w.Code, w.Body.String())
		}

		var resp TaskResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Task.Status != status {
			t.Errorf("status = %q, want %q", resp.Task.Status, status)
		}
		if resp.Started {
			t.Errorf("move to %s reported Started=true; that column starts nothing", status)
		}
	}
}

// Review reads the worktree the implementation wrote, so a task that was never
// implemented has nothing for the reviewer to look at.
func TestMoveToReviewWithoutImplementationIsRejected(t *testing.T) {
	srv := newTestServer(t)
	created := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "Never implemented"})

	w := postJSON(t, srv, srv.handleTaskDetail, "/api/tasks/"+created.ID+"/move",
		MoveTaskRequest{Status: "in_review", Index: 0})

	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, "not been implemented") {
		t.Errorf("error should explain there is nothing to review, got: %s", body)
	}

	// The card still lands where it was dropped.
	after, err := srv.stateStore.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if after.Status != "in_review" {
		t.Errorf("status = %q, want in_review (move is preserved)", after.Status)
	}
}

func TestMoveRejectsUnknownStatus(t *testing.T) {
	srv := newTestServer(t)
	created := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "x"})

	w := postJSON(t, srv, srv.handleTaskDetail, "/api/tasks/"+created.ID+"/move",
		MoveTaskRequest{Status: "shipped"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

// Reordering within a column is a real operation even though status is
// unchanged, and must not be rejected as a no-op transition.
func TestMoveWithinSameColumnReorders(t *testing.T) {
	srv := newTestServer(t)
	a := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "a"})
	createTaskViaAPI(t, srv, CreateTaskRequest{Title: "b"})
	createTaskViaAPI(t, srv, CreateTaskRequest{Title: "c"})

	w := postJSON(t, srv, srv.handleTaskDetail, "/api/tasks/"+a.ID+"/move",
		MoveTaskRequest{Status: "todo", Index: 2})
	if w.Code != http.StatusOK {
		t.Fatalf("same-column move: status %d body=%s", w.Code, w.Body.String())
	}

	tasks, err := srv.stateStore.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if tasks[len(tasks)-1].ID != a.ID {
		t.Errorf("task 'a' should now be last in todo")
	}
}

func TestTaskDetailMethods(t *testing.T) {
	srv := newTestServer(t)
	created := createTaskViaAPI(t, srv, CreateTaskRequest{Title: "Original"})
	path := "/api/tasks/" + created.ID

	// PATCH
	var buf bytes.Buffer
	newTitle := "Renamed"
	_ = json.NewEncoder(&buf).Encode(UpdateTaskRequest{Title: &newTitle})
	req := httptest.NewRequest(http.MethodPatch, path, &buf)
	w := httptest.NewRecorder()
	srv.handleTaskDetail(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch: status %d body=%s", w.Code, w.Body.String())
	}

	// GET reflects it
	req = httptest.NewRequest(http.MethodGet, path, nil)
	w = httptest.NewRecorder()
	srv.handleTaskDetail(w, req)
	var got state.Task
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Title != "Renamed" {
		t.Errorf("Title = %q, want Renamed", got.Title)
	}

	// DELETE
	req = httptest.NewRequest(http.MethodDelete, path, nil)
	w = httptest.NewRecorder()
	srv.handleTaskDetail(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("delete: status %d, want 204", w.Code)
	}

	// Gone
	req = httptest.NewRequest(http.MethodGet, path, nil)
	w = httptest.NewRecorder()
	srv.handleTaskDetail(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("get after delete: status %d, want 404", w.Code)
	}
}

func TestTaskDetailRequiresID(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/", nil)
	w := httptest.NewRecorder()
	srv.handleTaskDetail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

func TestUnknownTaskReturns404(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/ghost", nil)
	w := httptest.NewRecorder()
	srv.handleTaskDetail(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
}
