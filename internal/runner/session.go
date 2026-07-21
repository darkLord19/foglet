package runner

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/state"
	"github.com/google/uuid"
)

// StartSessionOptions configures the first run in a new session.
type StartSessionOptions struct {
	RepoName    string
	RepoPath    string
	Branch      string
	Tool        string
	Model       string
	Prompt      string
	AutoPR      bool
	SetupCmd    string
	Validate    bool
	ValidateCmd string
	BaseBranch  string
	CommitMsg   string
	PRTitle     string
}

// StartSession creates a new session (branch/worktree) and executes the initial prompt.
func (r *Runner) StartSession(opts StartSessionOptions) (state.Session, state.Run, error) {
	session, run, execOpts, err := r.prepareSession(opts)
	if err != nil {
		return state.Session{}, state.Run{}, err
	}
	err = r.executeSessionRun(session, run, execOpts)
	return r.loadSessionAndRun(session.ID, run.ID, err)
}

// StartSessionAsync creates a new session and starts the initial run in the background.
func (r *Runner) StartSessionAsync(opts StartSessionOptions) (state.Session, state.Run, error) {
	session, run, execOpts, err := r.prepareSession(opts)
	if err != nil {
		return state.Session{}, state.Run{}, err
	}
	go func(s state.Session, ru state.Run, eo sessionRunOptions) {
		_ = r.executeSessionRun(s, ru, eo)
	}(session, run, execOpts)
	return session, run, nil
}

// ContinueSession appends one follow-up run to an existing session.
func (r *Runner) ContinueSession(sessionID, prompt string) (state.Run, error) {
	session, run, execOpts, err := r.prepareFollowUpRun(sessionID, prompt)
	if err != nil {
		return state.Run{}, err
	}
	err = r.executeSessionRun(session, run, execOpts)
	updatedRun, found, runErr := r.runs.GetRun(run.ID)
	if runErr != nil {
		return state.Run{}, runErr
	}
	if !found {
		return state.Run{}, fmt.Errorf("run %q disappeared", run.ID)
	}
	if err != nil {
		return updatedRun, err
	}
	return updatedRun, nil
}

// ContinueSessionAsync appends one follow-up run and executes it in the background.
func (r *Runner) ContinueSessionAsync(sessionID, prompt string) (state.Run, error) {
	session, run, execOpts, err := r.prepareFollowUpRun(sessionID, prompt)
	if err != nil {
		return state.Run{}, err
	}
	go func(s state.Session, ru state.Run, eo sessionRunOptions) {
		_ = r.executeSessionRun(s, ru, eo)
	}(session, run, execOpts)
	return run, nil
}

// ForkSessionOptions configures a fork from an existing session.
type ForkSessionOptions struct {
	Branch      string
	Prompt      string
	Tool        string
	Model       string
	AutoPR      bool
	HasAutoPR   bool
	SetupCmd    string
	Validate    bool
	ValidateCmd string
	BaseBranch  string
	CommitMsg   string
	PRTitle     string
}

// ForkSession creates a new session from an existing one and runs immediately.
func (r *Runner) ForkSession(sourceSessionID string, opts ForkSessionOptions) (state.Session, state.Run, error) {
	startOpts, sourceSession, err := r.prepareForkSession(sourceSessionID, opts)
	if err != nil {
		return state.Session{}, state.Run{}, err
	}

	session, run, runErr := r.StartSession(startOpts)
	if runErr != nil {
		return session, run, runErr
	}
	r.annotateForkRun(run.ID, sourceSession)
	return session, run, nil
}

// ForkSessionAsync creates a new session from an existing one and starts it in the background.
func (r *Runner) ForkSessionAsync(sourceSessionID string, opts ForkSessionOptions) (state.Session, state.Run, error) {
	startOpts, sourceSession, err := r.prepareForkSession(sourceSessionID, opts)
	if err != nil {
		return state.Session{}, state.Run{}, err
	}

	session, run, runErr := r.StartSessionAsync(startOpts)
	if runErr != nil {
		return session, run, runErr
	}
	r.annotateForkRun(run.ID, sourceSession)
	return session, run, nil
}

func (r *Runner) prepareSession(opts StartSessionOptions) (state.Session, state.Run, sessionRunOptions, error) {
	if r.runs == nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("state store not configured")
	}

	opts.RepoName = strings.TrimSpace(opts.RepoName)
	opts.RepoPath = strings.TrimSpace(opts.RepoPath)
	opts.Branch = strings.TrimSpace(opts.Branch)
	opts.Tool = strings.TrimSpace(opts.Tool)
	opts.Model = strings.TrimSpace(opts.Model)
	opts.Prompt = strings.TrimSpace(opts.Prompt)
	opts.SetupCmd = strings.TrimSpace(opts.SetupCmd)
	opts.ValidateCmd = strings.TrimSpace(opts.ValidateCmd)
	opts.BaseBranch = strings.TrimSpace(opts.BaseBranch)
	opts.CommitMsg = strings.TrimSpace(opts.CommitMsg)

	switch {
	case opts.RepoName == "":
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("repo name is required")
	case opts.RepoPath == "":
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("repo path is required")
	case opts.Branch == "":
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("branch is required")
	case opts.Tool == "":
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("tool is required")
	case opts.Prompt == "":
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("prompt is required")
	case opts.BaseBranch == "":
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("base branch is required")
	}

	runID := uuid.New().String()
	worktreeName := runWorktreeName(opts.Branch, runID)
	worktreePath, err := r.createWorktreePathWithName(opts.RepoPath, worktreeName, opts.Branch, opts.BaseBranch)
	if err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	now := time.Now().UTC()
	session := state.Session{
		ID:           uuid.New().String(),
		RepoName:     opts.RepoName,
		Branch:       opts.Branch,
		WorktreePath: worktreePath,
		Tool:         opts.Tool,
		Model:        opts.Model,
		AutoPR:       opts.AutoPR,
		Status:       "CREATED",
		Busy:         true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.runs.CreateSession(session); err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	run := state.Run{
		ID:           runID,
		SessionID:    session.ID,
		Prompt:       opts.Prompt,
		WorktreePath: worktreePath,
		State:        "CREATED",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.runs.CreateRun(run); err != nil {
		_ = r.runs.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	return session, run, sessionRunOptions{
		Prompt:      opts.Prompt,
		SetupCmd:    opts.SetupCmd,
		Validate:    opts.Validate,
		ValidateCmd: opts.ValidateCmd,
		BaseBranch:  opts.BaseBranch,
		CommitMsg:   opts.CommitMsg,
		PRTitle:     opts.PRTitle,
	}, nil
}

func (r *Runner) prepareFollowUpRun(sessionID, prompt string) (state.Session, state.Run, sessionRunOptions, error) {
	if r.runs == nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("state store not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	prompt = strings.TrimSpace(prompt)
	if sessionID == "" {
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("session id is required")
	}
	if prompt == "" {
		return state.Session{}, state.Run{}, sessionRunOptions{}, errors.New("prompt is required")
	}

	session, found, err := r.runs.GetSession(sessionID)
	if err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	if !found {
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("session %q: %w", sessionID, state.ErrNotFound)
	}
	if session.Busy {
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("session %q is busy", sessionID)
	}
	if err := r.runs.SetSessionBusy(session.ID, true); err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	worktreePath := strings.TrimSpace(session.WorktreePath)
	if worktreePath == "" {
		_ = r.runs.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("session %q has no worktree path", session.ID)
	}

	runID := uuid.New().String()

	now := time.Now().UTC()
	run := state.Run{
		ID:           runID,
		SessionID:    session.ID,
		Prompt:       prompt,
		WorktreePath: worktreePath,
		State:        "CREATED",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.runs.CreateRun(run); err != nil {
		_ = r.runs.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	if err := r.runs.UpdateSessionStatus(session.ID, "CREATED"); err != nil {
		_ = r.runs.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	repo, _, _ := r.repos.GetRepoByName(session.RepoName)
	baseBranch := strings.TrimSpace(repo.DefaultBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}
	return session, run, sessionRunOptions{
		Prompt:     prompt,
		BaseBranch: baseBranch,
	}, nil
}

func (r *Runner) prepareForkSession(sourceSessionID string, opts ForkSessionOptions) (StartSessionOptions, state.Session, error) {
	if r.runs == nil {
		return StartSessionOptions{}, state.Session{}, errors.New("state store not configured")
	}

	sourceSessionID = strings.TrimSpace(sourceSessionID)
	opts.Branch = strings.TrimSpace(opts.Branch)
	opts.Prompt = strings.TrimSpace(opts.Prompt)
	opts.Tool = strings.TrimSpace(opts.Tool)
	opts.Model = strings.TrimSpace(opts.Model)
	opts.SetupCmd = strings.TrimSpace(opts.SetupCmd)
	opts.ValidateCmd = strings.TrimSpace(opts.ValidateCmd)
	opts.BaseBranch = strings.TrimSpace(opts.BaseBranch)
	opts.CommitMsg = strings.TrimSpace(opts.CommitMsg)

	switch {
	case sourceSessionID == "":
		return StartSessionOptions{}, state.Session{}, errors.New("source session id is required")
	case opts.Branch == "":
		return StartSessionOptions{}, state.Session{}, errors.New("branch is required")
	case opts.Prompt == "":
		return StartSessionOptions{}, state.Session{}, errors.New("prompt is required")
	}

	sourceSession, found, err := r.runs.GetSession(sourceSessionID)
	if err != nil {
		return StartSessionOptions{}, state.Session{}, err
	}
	if !found {
		return StartSessionOptions{}, state.Session{}, fmt.Errorf("session %q: %w", sourceSessionID, state.ErrNotFound)
	}
	if sourceSession.Busy {
		return StartSessionOptions{}, state.Session{}, fmt.Errorf("session %q is busy", sourceSessionID)
	}
	sourceWorktreePath := strings.TrimSpace(sourceSession.WorktreePath)
	if sourceWorktreePath == "" {
		return StartSessionOptions{}, state.Session{}, fmt.Errorf("session %q has no worktree path", sourceSessionID)
	}

	repo, found, err := r.repos.GetRepoByName(sourceSession.RepoName)
	if err != nil {
		return StartSessionOptions{}, state.Session{}, err
	}
	if !found {
		return StartSessionOptions{}, state.Session{}, fmt.Errorf("repo %q: %w", sourceSession.RepoName, state.ErrNotFound)
	}
	if strings.TrimSpace(repo.BaseWorktreePath) == "" {
		return StartSessionOptions{}, state.Session{}, fmt.Errorf("repo %q has no base worktree path", sourceSession.RepoName)
	}

	tool := opts.Tool
	if tool == "" {
		tool = sourceSession.Tool
	}
	if tool == "" {
		return StartSessionOptions{}, state.Session{}, errors.New("tool is required")
	}

	model := opts.Model
	if model == "" {
		model = sourceSession.Model
	}

	autoPR := sourceSession.AutoPR
	if opts.HasAutoPR {
		autoPR = opts.AutoPR
	}

	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(repo.DefaultBranch)
		if baseBranch == "" {
			baseBranch = "main"
		}
	}

	finalPrompt := opts.Prompt
	if summary, err := r.generateForkSummary(sourceSession, opts.Prompt, tool); err == nil {
		summary = strings.TrimSpace(summary)
		if summary != "" {
			finalPrompt = strings.TrimSpace(opts.Prompt) + "\n\nContext from source session:\n" + summary
		}
	}

	return StartSessionOptions{
		RepoName:    sourceSession.RepoName,
		RepoPath:    sourceWorktreePath,
		Branch:      opts.Branch,
		Tool:        tool,
		Model:       model,
		Prompt:      finalPrompt,
		AutoPR:      autoPR,
		SetupCmd:    opts.SetupCmd,
		Validate:    opts.Validate,
		ValidateCmd: opts.ValidateCmd,
		BaseBranch:  baseBranch,
		CommitMsg:   opts.CommitMsg,
		PRTitle:     opts.PRTitle,
	}, sourceSession, nil
}

func (r *Runner) annotateForkRun(runID string, sourceSession state.Session) {
	if r.runs == nil {
		return
	}
	_ = r.runs.AppendRunEvent(state.RunEvent{
		RunID:   runID,
		Type:    "fork",
		Message: fmt.Sprintf("Forked from session %s on branch %s", sourceSession.ID, sourceSession.Branch),
	})
}

func (r *Runner) loadSessionAndRun(sessionID, runID string, runErr error) (state.Session, state.Run, error) {
	updatedSession, found, sessionErr := r.runs.GetSession(sessionID)
	if sessionErr != nil {
		return state.Session{}, state.Run{}, sessionErr
	}
	if !found {
		return state.Session{}, state.Run{}, fmt.Errorf("session %q disappeared", sessionID)
	}
	updatedRun, found, getRunErr := r.runs.GetRun(runID)
	if getRunErr != nil {
		return state.Session{}, state.Run{}, getRunErr
	}
	if !found {
		return state.Session{}, state.Run{}, fmt.Errorf("run %q disappeared", runID)
	}
	if runErr != nil {
		return updatedSession, updatedRun, runErr
	}
	return updatedSession, updatedRun, nil
}

// GetSession returns one session by id.
func (r *Runner) GetSession(id string) (state.Session, bool, error) {
	if r.runs == nil {
		return state.Session{}, false, errors.New("state store not configured")
	}
	return r.runs.GetSession(id)
}

// ListSessions returns all sessions ordered by updated time.
func (r *Runner) ListSessions() ([]state.Session, error) {
	if r.runs == nil {
		return nil, errors.New("state store not configured")
	}
	return r.runs.ListSessions()
}

// ListSessionRuns returns runs in one session.
func (r *Runner) ListSessionRuns(sessionID string) ([]state.Run, error) {
	if r.runs == nil {
		return nil, errors.New("state store not configured")
	}
	return r.runs.ListRuns(sessionID)
}

// ListRunEvents returns run events in chronological order.
func (r *Runner) ListRunEvents(runID string, limit int) ([]state.RunEvent, error) {
	if r.runs == nil {
		return nil, errors.New("state store not configured")
	}
	return r.runs.ListRunEvents(runID, limit)
}

// CancelSessionLatestRun requests cancellation for the active latest run in a session.
func (r *Runner) CancelSessionLatestRun(sessionID string) (state.Run, error) {
	if r.runs == nil {
		return state.Run{}, errors.New("state store not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return state.Run{}, errors.New("session id is required")
	}

	session, found, err := r.runs.GetSession(sessionID)
	if err != nil {
		return state.Run{}, err
	}
	if !found {
		return state.Run{}, fmt.Errorf("session %q: %w", sessionID, state.ErrNotFound)
	}

	latest, found, err := r.runs.GetLatestRun(session.ID)
	if err != nil {
		return state.Run{}, err
	}
	if !found {
		return state.Run{}, fmt.Errorf("session %q has no runs", sessionID)
	}

	r.mu.Lock()
	current, ok := r.active[session.ID]
	if !ok || current == nil {
		r.mu.Unlock()
		return state.Run{}, fmt.Errorf("latest run %q is not active", latest.ID)
	}
	if strings.TrimSpace(current.runID) != latest.ID {
		r.mu.Unlock()
		return state.Run{}, fmt.Errorf("only the latest active run can be canceled")
	}
	cancel := current.cancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	_ = r.runs.AppendRunEvent(state.RunEvent{
		RunID:   latest.ID,
		Type:    "cancel_requested",
		Message: "Cancellation requested by user",
	})
	return latest, nil
}

func (r *Runner) lookupConversationID(sessionID, currentRunID string) string {
	if r.runs == nil {
		return ""
	}
	runs, err := r.runs.ListRuns(sessionID)
	if err != nil {
		return ""
	}
	for _, run := range runs {
		if strings.TrimSpace(run.ID) == strings.TrimSpace(currentRunID) {
			continue
		}
		events, err := r.runs.ListRunEvents(run.ID, 200)
		if err != nil {
			continue
		}
		for i := len(events) - 1; i >= 0; i-- {
			event := events[i]
			if strings.TrimSpace(event.Type) != "ai_session" {
				continue
			}
			if sessionID := strings.TrimSpace(event.Data); sessionID != "" {
				return sessionID
			}
		}
	}
	return ""
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	var b bytes.Buffer
	b.WriteString(value[:max])
	b.WriteString("...")
	return b.String()
}

func extractCommitMessage(output string) string {
	startTag := "<commit_message>"
	endTag := "</commit_message>"

	startIdx := strings.LastIndex(output, startTag)
	if startIdx == -1 {
		return ""
	}

	content := output[startIdx+len(startTag):]
	before, _, ok := strings.Cut(content, endTag)
	if !ok {
		return strings.TrimSpace(content)
	}

	return strings.TrimSpace(before)
}
