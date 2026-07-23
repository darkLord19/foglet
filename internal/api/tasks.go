package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
	"github.com/google/uuid"
)

// CreateTaskRequest is the payload for POST /api/tasks.
type CreateTaskRequest struct {
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	Status     string `json:"status,omitempty"`
	Repo       string `json:"repo,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Model      string `json:"model,omitempty"`
	BaseBranch string `json:"base_branch,omitempty"`
}

// UpdateTaskRequest is the payload for PATCH /api/tasks/{id}.
type UpdateTaskRequest struct {
	Title      *string `json:"title,omitempty"`
	Body       *string `json:"body,omitempty"`
	Repo       *string `json:"repo,omitempty"`
	Tool       *string `json:"tool,omitempty"`
	Model      *string `json:"model,omitempty"`
	BaseBranch *string `json:"base_branch,omitempty"`
}

// MoveTaskRequest is the payload for POST /api/tasks/{id}/move.
//
// There is deliberately no "origin" field. Origin is not something a caller
// gets to assert — it is derived from which code path performed the move. This
// HTTP surface is bound to loopback and reached only by the desktop UI on this
// machine, so every move through it is task.OriginLocal. Tracker sync runs
// inside the daemon and calls the store directly with task.OriginRemote, which
// is what stops a teammate's drag in Linear or Jira from starting an agent here.
type MoveTaskRequest struct {
	Status string `json:"status"`
	Index  int    `json:"index"`
}

// TaskResponse wraps a task plus whether the last operation started an agent.
type TaskResponse struct {
	Task    state.Task `json:"task"`
	Started bool       `json:"started"`
	// Kind is "implement" or "review" when Started is true.
	Kind string `json:"kind,omitempty"`
	// SessionID is set when this call launched or attached a session.
	SessionID string `json:"session_id,omitempty"`
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTasks(w)
	case http.MethodPost:
		s.createTask(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tasks/"), "/")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "task id required")
		return
	}

	parts := strings.Split(path, "/")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		writeErr(w, http.StatusBadRequest, "task id required")
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.getTask(w, id)
		case http.MethodPatch:
			s.updateTask(w, r, id)
		case http.MethodDelete:
			s.deleteTask(w, id)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		switch parts[1] {
		case "move":
			s.moveTask(w, r, id)
			return
		case "start":
			s.startTask(w, id)
			return
		case "restore":
			s.restoreTask(w, id)
			return
		case "purge":
			s.purgeTask(w, id)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) listTasks(w http.ResponseWriter) {
	tasks, err := s.stateStore.ListTasks()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	status := task.StatusTodo
	if strings.TrimSpace(req.Status) != "" {
		parsed, err := task.ParseStatus(req.Status)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		status = parsed
	}

	if err := s.requireKnownRepo(req.Repo); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := s.stateStore.CreateTask(state.Task{
		ID:         uuid.NewString(),
		Title:      req.Title,
		Body:       strings.TrimSpace(req.Body),
		Status:     status.String(),
		RepoName:   strings.TrimSpace(req.Repo),
		Tool:       strings.TrimSpace(req.Tool),
		Model:      strings.TrimSpace(req.Model),
		BaseBranch: strings.TrimSpace(req.BaseBranch),
		Provider:   string(task.ProviderLocal),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, TaskResponse{Task: created})
}

func (s *Server) getTask(w http.ResponseWriter, id string) {
	t, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, t)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	t, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}

	if req.Title != nil {
		t.Title = strings.TrimSpace(*req.Title)
	}
	if req.Body != nil {
		t.Body = strings.TrimSpace(*req.Body)
	}
	if req.Repo != nil {
		if err := s.requireKnownRepo(*req.Repo); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		t.RepoName = strings.TrimSpace(*req.Repo)
	}
	if req.Tool != nil {
		t.Tool = strings.TrimSpace(*req.Tool)
	}
	if req.Model != nil {
		t.Model = strings.TrimSpace(*req.Model)
	}
	if req.BaseBranch != nil {
		t.BaseBranch = strings.TrimSpace(*req.BaseBranch)
	}

	if err := s.stateStore.UpdateTask(t); err != nil {
		s.writeTaskErr(w, err)
		return
	}

	updated, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, TaskResponse{Task: updated})
}

// deleteTask moves a task to trash rather than destroying it. If the linked
// session still has an active run, that run is stopped — trashing a card should
// not leave an agent working on discarded intent. The worktree is deliberately
// kept: the task stays recoverable until retention expires (see the trash
// janitor), at which point the worktree and branch are reclaimed.
func (s *Server) deleteTask(w http.ResponseWriter, id string) {
	current, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}

	s.stopSessionBestEffort(current.SessionID)

	if err := s.stateStore.TrashTask(id); err != nil {
		s.writeTaskErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// restoreTask brings a trashed task back onto the board.
func (s *Server) restoreTask(w http.ResponseWriter, id string) {
	if err := s.stateStore.RestoreTask(id); err != nil {
		s.writeTaskErr(w, err)
		return
	}
	task, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, TaskResponse{Task: task})
}

// purgeTask permanently deletes a trashed task now, without waiting for
// retention, reclaiming its session's worktree and branch first. This is the
// "delete forever" affordance in the trash view.
func (s *Server) purgeTask(w http.ResponseWriter, id string) {
	current, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}

	s.stopSessionBestEffort(current.SessionID)
	if current.SessionID != "" {
		if err := s.runner.RemoveSessionArtifacts(current.SessionID); err != nil {
			log.Printf("purge task %s: %v", id, err)
		}
	}

	if err := s.stateStore.DeleteTask(id); err != nil {
		s.writeTaskErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleTasksTrash lists trashed tasks for the trash view.
func (s *Server) handleTasksTrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	tasks, err := s.stateStore.ListTrashedTasks()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, tasks)
}

// stopSessionBestEffort cancels a session's active run if one exists. A session
// with nothing running (already finished, or never started) is not an error —
// there is simply nothing to stop.
func (s *Server) stopSessionBestEffort(sessionID string) {
	if sessionID == "" {
		return
	}
	if _, err := s.runner.CancelSessionLatestRun(sessionID); err != nil {
		// The common case: the session had no active run to cancel.
		log.Printf("stop session %s on trash: %v", sessionID, err)
	}
}

// moveTask repositions a card and, when the move warrants it, launches the
// agent. See MoveTaskRequest for why origin is hardcoded rather than read from
// the payload.
func (s *Server) moveTask(w http.ResponseWriter, r *http.Request, id string) {
	var req MoveTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	target, err := task.ParseStatus(req.Status)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	current, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}

	from, err := task.ParseStatus(current.Status)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Reordering inside a column is a legitimate move even though the status
	// is unchanged, so a same-column drop skips the transition check.
	if from != target {
		if err := task.CanTransition(from, target); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := s.stateStore.MoveTask(id, target.String(), req.Index); err != nil {
		s.writeTaskErr(w, err)
		return
	}

	resp := TaskResponse{}
	if kind, ok := task.AutoStarts(from, target, task.OriginLocal); ok {
		sessionID, err := s.startTaskWork(id, kind)
		if err != nil {
			// The move succeeded; only the launch failed. Report it without
			// rolling the card back — the user can see it landed and retry.
			writeErr(w, http.StatusConflict, fmt.Sprintf("task moved but agent did not start: %v", err))
			return
		}
		resp.Started = true
		resp.Kind = string(kind)
		resp.SessionID = sessionID
	}

	moved, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}
	resp.Task = moved
	s.writeJSON(w, http.StatusOK, resp)
}

// startTask launches the agent for a task explicitly. This is the affordance
// offered when a card arrives in a working column from a remote tracker, where
// auto-start is deliberately withheld.
//
// The kind is inferred from the column the task currently sits in, so the
// button does whatever the card's position implies.
func (s *Server) startTask(w http.ResponseWriter, id string) {
	t, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}

	kind := task.WorkImplement
	if t.Status == task.StatusInReview.String() {
		kind = task.WorkReview
	}

	sessionID, err := s.startTaskWork(id, kind)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}

	updated, err := s.stateStore.GetTask(id)
	if err != nil {
		s.writeTaskErr(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, TaskResponse{
		Task: updated, Started: true, Kind: string(kind), SessionID: sessionID,
	})
}

// startTaskWork turns a task into running work. Both the drag path and the
// explicit Start button go through here.
//
// The two kinds differ in more than prompt. Implementation opens a new session,
// which creates a branch and a worktree. Review must NOT do that — it has to
// read the code the implementation just wrote, so it appends a follow-up run to
// the existing session and inherits its worktree.
func (s *Server) startTaskWork(taskID string, kind task.WorkKind) (string, error) {
	t, err := s.stateStore.GetTask(taskID)
	if err != nil {
		return "", err
	}

	if kind == task.WorkReview {
		return s.startTaskReview(t)
	}

	if strings.TrimSpace(t.RepoName) == "" {
		return "", errors.New("task has no repository set")
	}

	prompt := t.Title
	if body := strings.TrimSpace(t.Body); body != "" {
		prompt = t.Title + "\n\n" + body
	}

	// Launch owns repo, tool, branch and base-branch resolution, so the board and
	// the sessions API can no longer drift apart on those rules.
	//
	// The board still cannot request AutoPR, SetupCmd, Validate, ValidateCmd,
	// CommitMsg or PRTitle — not because this path drops them any more, but
	// because state.Task has no columns to carry them. Launch accepts all six;
	// exposing them on a card needs a schema change.
	session, _, err := s.runner.Launch(runner.LaunchRequest{
		Entrypoint: "task",
		RepoName:   t.RepoName,
		Prompt:     prompt,
		Tool:       t.Tool,
		Model:      t.Model,
		BaseBranch: t.BaseBranch,
		Async:      true,
	})
	if err != nil {
		return "", err
	}

	if err := s.stateStore.LinkTaskSession(taskID, session.ID); err != nil {
		return session.ID, fmt.Errorf("session %s started but not linked: %w", session.ID, err)
	}
	return session.ID, nil
}

// startTaskReview appends a read-only review run to the task's existing
// session, so the reviewer agent sees the implementation's worktree.
func (s *Server) startTaskReview(t state.Task) (string, error) {
	if strings.TrimSpace(t.SessionID) == "" {
		return "", errors.New("nothing to review: this task has not been implemented yet")
	}

	session, found, err := s.runner.GetSession(t.SessionID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("session %s no longer exists", t.SessionID)
	}
	if session.Busy {
		return "", errors.New("the implementation is still running; wait for it to finish")
	}

	if _, err := s.runner.ContinueSessionAsync(t.SessionID, task.ReviewPrompt(t.Title, t.Body)); err != nil {
		return "", err
	}
	return t.SessionID, nil
}

// requireKnownRepo validates an optional repo name against the store. An empty
// name is allowed: a task can be written down before you know where it runs.
func (s *Server) requireKnownRepo(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	_, found, err := s.stateStore.GetRepoByName(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("unknown repo: %s", name)
	}
	return nil
}

func (s *Server) writeTaskErr(w http.ResponseWriter, err error) {
	if errors.Is(err, state.ErrTaskNotFound) {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, err.Error())
}
