package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/editor"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/toolcfg"
)

// dangerousShellChars contains characters that enable shell injection when
// passed through sh -c. We reject commands containing any of these.
var dangerousShellChars = []string{";", "||", "&&", "|", "`", "$(", "${", ">", "<", "\n", "\r"}

// validateShellCommand rejects commands containing dangerous shell metacharacters.
func validateShellCommand(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	for _, ch := range dangerousShellChars {
		if strings.Contains(cmd, ch) {
			return fmt.Errorf("validate_cmd contains forbidden character sequence %q", ch)
		}
	}
	return nil
}

// CreateSessionRequest is the payload for POST /api/sessions.
type CreateSessionRequest struct {
	Repo        string `json:"repo"`
	Tool        string `json:"tool,omitempty"`
	Model       string `json:"model,omitempty"`
	Prompt      string `json:"prompt"`
	BranchName  string `json:"branch_name,omitempty"`
	AutoPR      *bool  `json:"autopr,omitempty"`
	SetupCmd    string `json:"setup_cmd,omitempty"`
	Validate    bool   `json:"validate,omitempty"`
	ValidateCmd string `json:"validate_cmd,omitempty"`
	BaseBranch  string `json:"base_branch,omitempty"`
	CommitMsg   string `json:"commit_msg,omitempty"`
	Async       *bool  `json:"async,omitempty"`
	PRTitle     string `json:"pr_title,omitempty"`
}

// FollowUpRunRequest is the payload for POST /api/sessions/{id}/runs.
type FollowUpRunRequest struct {
	Prompt string `json:"prompt"`
	Async  *bool  `json:"async,omitempty"`
}

// ForkSessionRequest is the payload for POST /api/sessions/{id}/fork.
type ForkSessionRequest struct {
	Prompt      string `json:"prompt"`
	BranchName  string `json:"branch_name,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Model       string `json:"model,omitempty"`
	AutoPR      *bool  `json:"autopr,omitempty"`
	SetupCmd    string `json:"setup_cmd,omitempty"`
	Validate    bool   `json:"validate,omitempty"`
	ValidateCmd string `json:"validate_cmd,omitempty"`
	BaseBranch  string `json:"base_branch,omitempty"`
	CommitMsg   string `json:"commit_msg,omitempty"`
	Async       *bool  `json:"async,omitempty"`
	PRTitle     string `json:"pr_title,omitempty"`
}

type createSessionResponse struct {
	Session state.Session `json:"session"`
	Run     state.Run     `json:"run"`
}

type asyncCreateSessionResponse struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
	Status    string `json:"status"`
}

type sessionDetailResponse struct {
	Session state.Session `json:"session"`
	Runs    []state.Run   `json:"runs"`
}

type sessionSummary struct {
	state.Session
	LatestRun *state.Run `json:"latest_run,omitempty"`
}

type sessionDiffResponse struct {
	BaseBranch   string `json:"base_branch"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
	Stat         string `json:"stat"`
	Patch        string `json:"patch"`
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSessions(w)
	case http.MethodPost:
		s.createSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}
	parts := strings.Split(path, "/")
	sessionID := strings.TrimSpace(parts[0])
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.getSession(w, sessionID)
		return
	}

	if len(parts) >= 2 && parts[1] == "runs" {
		switch {
		case len(parts) == 2 && r.Method == http.MethodGet:
			s.listSessionRuns(w, sessionID)
			return
		case len(parts) == 2 && r.Method == http.MethodPost:
			s.createFollowUpRun(w, r, sessionID)
			return
		case len(parts) == 4 && parts[3] == "events" && r.Method == http.MethodGet:
			s.listRunEvents(w, r, sessionID, parts[2])
			return
		case len(parts) == 4 && parts[3] == "stream" && r.Method == http.MethodGet:
			s.streamRunEvents(w, r, sessionID, parts[2])
			return
		}
	}
	if len(parts) == 2 {
		switch {
		case parts[1] == "cancel" && r.Method == http.MethodPost:
			s.cancelSessionRun(w, sessionID)
			return
		case parts[1] == "fork" && r.Method == http.MethodPost:
			s.createForkSession(w, r, sessionID)
			return
		case parts[1] == "diff" && r.Method == http.MethodGet:
			s.getSessionDiff(w, sessionID)
			return
		case parts[1] == "open" && r.Method == http.MethodPost:
			s.openSessionWorktree(w, sessionID)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) listSessions(w http.ResponseWriter) {
	sessions, err := s.runner.ListSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]sessionSummary, 0, len(sessions))
	for _, sess := range sessions {
		var latest *state.Run
		if run, found, err := s.stateStore.GetLatestRun(sess.ID); err == nil && found {
			runCopy := run
			latest = &runCopy
		}
		out = append(out, sessionSummary{
			Session:   sess,
			LatestRun: latest,
		})
	}

	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.Repo = strings.TrimSpace(req.Repo)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Repo == "" || req.Prompt == "" {
		http.Error(w, "repo and prompt are required", http.StatusBadRequest)
		return
	}
	if err := validateShellCommand(req.ValidateCmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repo, found, err := s.stateStore.GetRepoByName(req.Repo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, fmt.Sprintf("unknown repo: %s", req.Repo), http.StatusBadRequest)
		return
	}

	tool, err := toolcfg.ResolveTool(req.Tool, s.stateStore, "api")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	branch, err := s.runner.ResolveBranch(repo.BaseWorktreePath, req.BranchName, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	autoPR := false
	if req.AutoPR != nil {
		autoPR = *req.AutoPR
	}
	async := true
	if req.Async != nil {
		async = *req.Async
	}

	baseBranch := strings.TrimSpace(req.BaseBranch)
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(repo.DefaultBranch)
	}
	if baseBranch == "" {
		baseBranch = "main"
	}

	opts := runner.StartSessionOptions{
		RepoName:    repo.Name,
		RepoPath:    repo.BaseWorktreePath,
		Branch:      branch,
		Tool:        tool,
		Model:       strings.TrimSpace(req.Model),
		Prompt:      req.Prompt,
		AutoPR:      autoPR,
		SetupCmd:    strings.TrimSpace(req.SetupCmd),
		Validate:    req.Validate,
		ValidateCmd: strings.TrimSpace(req.ValidateCmd),
		BaseBranch:  baseBranch,
		CommitMsg:   strings.TrimSpace(req.CommitMsg),
		PRTitle:     strings.TrimSpace(req.PRTitle),
	}

	if async {
		session, run, err := s.runner.StartSessionAsync(opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.writeJSON(w, http.StatusAccepted, asyncCreateSessionResponse{
			SessionID: session.ID,
			RunID:     run.ID,
			Status:    "accepted",
		})
		return
	}

	session, run, err := s.runner.StartSession(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, http.StatusOK, createSessionResponse{
		Session: session,
		Run:     run,
	})
}

func (s *Server) getSession(w http.ResponseWriter, sessionID string) {
	session, found, err := s.runner.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	runs, err := s.runner.ListSessionRuns(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, sessionDetailResponse{
		Session: session,
		Runs:    runs,
	})
}

func (s *Server) listSessionRuns(w http.ResponseWriter, sessionID string) {
	_, found, err := s.runner.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	runs, err := s.runner.ListSessionRuns(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, runs)
}

func (s *Server) createFollowUpRun(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req FollowUpRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	async := true
	if req.Async != nil {
		async = *req.Async
	}
	if async {
		run, err := s.runner.ContinueSessionAsync(sessionID, req.Prompt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.writeJSON(w, http.StatusAccepted, map[string]string{
			"run_id":  run.ID,
			"status":  "accepted",
			"session": run.SessionID,
		})
		return
	}

	run, err := s.runner.ContinueSession(sessionID, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, http.StatusOK, run)
}

func (s *Server) createForkSession(w http.ResponseWriter, r *http.Request, sourceSessionID string) {
	var req ForkSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	if err := validateShellCommand(req.ValidateCmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sourceSession, found, err := s.runner.GetSession(sourceSessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	branch, err := s.runner.ResolveBranch(sourceSession.WorktreePath, req.BranchName, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tool := strings.TrimSpace(req.Tool)
	if tool != "" {
		if _, err := ai.GetTool(tool); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	async := true
	if req.Async != nil {
		async = *req.Async
	}

	opts := runner.ForkSessionOptions{
		Branch:      branch,
		Prompt:      req.Prompt,
		Tool:        tool,
		Model:       strings.TrimSpace(req.Model),
		SetupCmd:    strings.TrimSpace(req.SetupCmd),
		Validate:    req.Validate,
		ValidateCmd: strings.TrimSpace(req.ValidateCmd),
		BaseBranch:  strings.TrimSpace(req.BaseBranch),
		CommitMsg:   strings.TrimSpace(req.CommitMsg),
		PRTitle:     strings.TrimSpace(req.PRTitle),
	}
	if req.AutoPR != nil {
		opts.HasAutoPR = true
		opts.AutoPR = *req.AutoPR
	}
	if strings.TrimSpace(opts.Tool) == "" {
		opts.Tool = sourceSession.Tool
	}

	if async {
		session, run, err := s.runner.ForkSessionAsync(sourceSessionID, opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.writeJSON(w, http.StatusAccepted, asyncCreateSessionResponse{
			SessionID: session.ID,
			RunID:     run.ID,
			Status:    "accepted",
		})
		return
	}

	session, run, err := s.runner.ForkSession(sourceSessionID, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, http.StatusOK, createSessionResponse{
		Session: session,
		Run:     run,
	})
}

func (s *Server) listRunEvents(w http.ResponseWriter, r *http.Request, sessionID, runID string) {
	session, found, err := s.runner.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	runs, err := s.runner.ListSessionRuns(session.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allowed := false
	for _, run := range runs {
		if run.ID == runID {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "run not found in session", http.StatusNotFound)
		return
	}

	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	events, err := s.runner.ListRunEvents(runID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, events)
}

func (s *Server) streamRunEvents(w http.ResponseWriter, r *http.Request, sessionID, runID string) {
	session, found, err := s.runner.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	runs, err := s.runner.ListSessionRuns(session.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allowed := false
	for _, run := range runs {
		if run.ID == runID {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "run not found in session", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	cursor := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			cursor = parsed
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ticker := time.NewTicker(700 * time.Millisecond)
	defer ticker.Stop()

	for {
		events, err := s.runner.ListRunEvents(runID, 2000)
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
			flusher.Flush()
			return
		}

		for _, event := range events {
			if event.ID <= cursor {
				continue
			}
			payload, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\n", event.ID)
			fmt.Fprintf(w, "event: run_event\n")
			fmt.Fprintf(w, "data: %s\n\n", payload)
			cursor = event.ID
		}
		flusher.Flush()

		run, found, err := s.stateStore.GetRun(runID)
		if err == nil && found && isTerminalRunState(run.State) {
			fmt.Fprintf(w, "event: done\ndata: %q\n\n", run.State)
			flusher.Flush()
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func isTerminalRunState(stateName string) bool {
	switch strings.TrimSpace(stateName) {
	case "COMPLETED", "FAILED", "CANCELLED":
		return true
	default:
		return false
	}
}

func (s *Server) cancelSessionRun(w http.ResponseWriter, sessionID string) {
	run, err := s.runner.CancelSessionLatestRun(sessionID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	s.writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "cancel_requested",
		"run_id": run.ID,
	})
}

func (s *Server) getSessionDiff(w http.ResponseWriter, sessionID string) {
	stat, patch, err := s.runner.SessionDiff(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, found, err := s.runner.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	repo, found, err := s.stateStore.GetRepoByName(session.RepoName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	worktreePath := strings.TrimSpace(session.WorktreePath)
	if latest, found, err := s.stateStore.GetLatestRun(session.ID); err == nil && found && strings.TrimSpace(latest.WorktreePath) != "" {
		worktreePath = strings.TrimSpace(latest.WorktreePath)
	}

	s.writeJSON(w, http.StatusOK, sessionDiffResponse{
		BaseBranch:   repo.DefaultBranch,
		Branch:       session.Branch,
		WorktreePath: worktreePath,
		Stat:         stat,
		Patch:        patch,
	})
}

func (s *Server) openSessionWorktree(w http.ResponseWriter, sessionID string) {
	session, found, err := s.runner.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	worktreePath := strings.TrimSpace(session.WorktreePath)
	if latest, found, err := s.stateStore.GetLatestRun(session.ID); err == nil && found && strings.TrimSpace(latest.WorktreePath) != "" {
		worktreePath = strings.TrimSpace(latest.WorktreePath)
	}
	if worktreePath == "" {
		http.Error(w, "session has no worktree path", http.StatusBadRequest)
		return
	}

	pref := preferredEditorForTool(session.Tool)
	ed, err := editor.Detect(pref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ed.Open(worktreePath, true); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"status":        "opened",
		"editor":        ed.Name(),
		"worktree_path": worktreePath,
	})
}

func preferredEditorForTool(toolName string) string {
	switch strings.TrimSpace(toolName) {
	case "cursor":
		return "cursor"
	case "claude", "claude-code":
		return "claudecode"
	default:
		return ""
	}
}


