package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/proc"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
	"github.com/google/uuid"
)

var nonWorktreeNameChar = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type activeRun struct {
	sessionID string
	runID     string
	cancel    context.CancelFunc
}

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
	updatedRun, found, runErr := r.state.GetRun(run.ID)
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

func (r *Runner) prepareSession(opts StartSessionOptions) (state.Session, state.Run, sessionRunOptions, error) {
	if r.state == nil {
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
	}

	runID := uuid.New().String()
	worktreeName := runWorktreeName(opts.Branch, runID)
	worktreePath, err := r.createWorktreePathWithName(opts.RepoPath, worktreeName, opts.Branch)
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
		Status:       string(task.StateCreated),
		Busy:         true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.state.CreateSession(session); err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	run := state.Run{
		ID:           runID,
		SessionID:    session.ID,
		Prompt:       opts.Prompt,
		WorktreePath: worktreePath,
		State:        string(task.StateCreated),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.state.CreateRun(run); err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	return session, run, sessionRunOptions{
		Prompt:      opts.Prompt,
		SetupCmd:    opts.SetupCmd,
		Validate:    opts.Validate,
		ValidateCmd: opts.ValidateCmd,
		BaseBranch:  opts.BaseBranch,
		CommitMsg:   opts.CommitMsg,
	}, nil
}

func (r *Runner) prepareFollowUpRun(sessionID, prompt string) (state.Session, state.Run, sessionRunOptions, error) {
	if r.state == nil {
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

	session, found, err := r.state.GetSession(sessionID)
	if err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	if !found {
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("session %q not found", sessionID)
	}
	if session.Busy {
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("session %q is busy", sessionID)
	}
	if err := r.state.SetSessionBusy(session.ID, true); err != nil {
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	repo, found, err := r.state.GetRepoByName(session.RepoName)
	if err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	if !found {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("repo %q not found", session.RepoName)
	}
	if strings.TrimSpace(repo.BaseWorktreePath) == "" {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, fmt.Errorf("repo %q has no base worktree path", session.RepoName)
	}

	if err := r.detachWorktreeHead(session.WorktreePath); err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	runID := uuid.New().String()
	worktreeName := runWorktreeName(session.Branch, runID)
	worktreePath, err := r.createWorktreePathWithName(repo.BaseWorktreePath, worktreeName, session.Branch)
	if err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	if err := r.state.SetSessionWorktreePath(session.ID, worktreePath); err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	session.WorktreePath = worktreePath

	now := time.Now().UTC()
	run := state.Run{
		ID:           runID,
		SessionID:    session.ID,
		Prompt:       prompt,
		WorktreePath: worktreePath,
		State:        string(task.StateCreated),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.state.CreateRun(run); err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}
	if err := r.state.UpdateSessionStatus(session.ID, string(task.StateCreated)); err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	baseBranch := strings.TrimSpace(repo.DefaultBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}
	return session, run, sessionRunOptions{
		Prompt:     prompt,
		BaseBranch: baseBranch,
	}, nil
}

func (r *Runner) loadSessionAndRun(sessionID, runID string, runErr error) (state.Session, state.Run, error) {
	updatedSession, found, sessionErr := r.state.GetSession(sessionID)
	if sessionErr != nil {
		return state.Session{}, state.Run{}, sessionErr
	}
	if !found {
		return state.Session{}, state.Run{}, fmt.Errorf("session %q disappeared", sessionID)
	}
	updatedRun, found, getRunErr := r.state.GetRun(runID)
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
	if r.state == nil {
		return state.Session{}, false, errors.New("state store not configured")
	}
	return r.state.GetSession(id)
}

// ListSessions returns all sessions ordered by updated time.
func (r *Runner) ListSessions() ([]state.Session, error) {
	if r.state == nil {
		return nil, errors.New("state store not configured")
	}
	return r.state.ListSessions()
}

// ListSessionRuns returns runs in one session.
func (r *Runner) ListSessionRuns(sessionID string) ([]state.Run, error) {
	if r.state == nil {
		return nil, errors.New("state store not configured")
	}
	return r.state.ListRuns(sessionID)
}

// ListRunEvents returns run events in chronological order.
func (r *Runner) ListRunEvents(runID string, limit int) ([]state.RunEvent, error) {
	if r.state == nil {
		return nil, errors.New("state store not configured")
	}
	return r.state.ListRunEvents(runID, limit)
}

// CancelSessionLatestRun requests cancellation for the active latest run in a session.
func (r *Runner) CancelSessionLatestRun(sessionID string) (state.Run, error) {
	if r.state == nil {
		return state.Run{}, errors.New("state store not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return state.Run{}, errors.New("session id is required")
	}

	session, found, err := r.state.GetSession(sessionID)
	if err != nil {
		return state.Run{}, err
	}
	if !found {
		return state.Run{}, fmt.Errorf("session %q not found", sessionID)
	}

	latest, found, err := r.state.GetLatestRun(session.ID)
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
	_ = r.state.AppendRunEvent(state.RunEvent{
		RunID:   latest.ID,
		Type:    "cancel_requested",
		Message: "Cancellation requested by user",
	})
	return latest, nil
}

type sessionRunOptions struct {
	Prompt      string
	SetupCmd    string
	Validate    bool
	ValidateCmd string
	BaseBranch  string
	CommitMsg   string
}

func (r *Runner) executeSessionRun(session state.Session, run state.Run, opts sessionRunOptions) (retErr error) {
	if r.state == nil {
		return errors.New("state store not configured")
	}
	if strings.TrimSpace(opts.BaseBranch) == "" {
		opts.BaseBranch = "main"
	}
	if strings.TrimSpace(run.WorktreePath) == "" {
		run.WorktreePath = strings.TrimSpace(session.WorktreePath)
	}
	if strings.TrimSpace(run.WorktreePath) == "" {
		return errors.New("run worktree path is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.registerActiveRun(session.ID, run.ID, cancel)
	defer func() {
		r.clearActiveRun(session.ID, run.ID)
		cancel()
		if err := r.detachWorktreeHead(run.WorktreePath); err != nil {
			_ = r.state.AppendRunEvent(state.RunEvent{
				RunID:   run.ID,
				Type:    "warning",
				Message: err.Error(),
			})
			if retErr == nil {
				retErr = err
			}
		}
		if err := r.state.SetSessionBusy(session.ID, false); err != nil && retErr == nil {
			retErr = err
		}
	}()

	fail := func(phase string, err error) error {
		terminalState := string(task.StateFailed)
		eventType := "error"
		message := phase + ": " + err.Error()
		if isCanceledError(err) {
			terminalState = string(task.StateCancelled)
			eventType = "cancelled"
			message = phase + ": canceled"
		}
		_ = r.state.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    eventType,
			Message: message,
		})
		_ = r.state.CompleteRun(run.ID, terminalState, "", "", err.Error())
		_ = r.updateSessionStatusIfLatest(session.ID, run.ID, terminalState)
		return err
	}

	if opts.SetupCmd != "" {
		if err := r.setRunPhase(session.ID, run.ID, string(task.StateSetup)); err != nil {
			return err
		}
		_ = r.state.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "setup",
			Message: "Running setup command",
		})
		if err := r.runShell(ctx, run.WorktreePath, opts.SetupCmd); err != nil {
			return fail("setup", err)
		}
	}

	if err := r.setRunPhase(session.ID, run.ID, string(task.StateAIRunning)); err != nil {
		return err
	}
	_ = r.state.AppendRunEvent(state.RunEvent{
		RunID:   run.ID,
		Type:    "ai_start",
		Message: "Running AI tool",
	})
	aiOutput, err := r.runTool(ctx, session.Tool, run.WorktreePath, opts.Prompt)
	if err != nil {
		return fail("ai", err)
	}
	if strings.TrimSpace(aiOutput) != "" {
		_ = r.state.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "ai_output",
			Message: truncate(aiOutput, 8000),
		})
	}

	if opts.Validate && opts.ValidateCmd != "" {
		if err := r.setRunPhase(session.ID, run.ID, string(task.StateValidating)); err != nil {
			return err
		}
		if err := r.runShell(ctx, run.WorktreePath, opts.ValidateCmd); err != nil {
			return fail("validate", err)
		}
	}

	if err := r.setRunPhase(session.ID, run.ID, string(task.StateCommitted)); err != nil {
		return err
	}
	commitSHA, commitMsg, changed, err := r.commitSessionChanges(ctx, session.Tool, run.WorktreePath, opts.Prompt, opts.CommitMsg)
	if err != nil {
		return fail("commit", err)
	}
	if !changed {
		_ = r.state.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "commit",
			Message: "No changes to commit",
		})
	}

	// Push only when PR mode is enabled or a PR already exists for this session.
	if changed && (session.AutoPR || strings.TrimSpace(session.PRURL) != "") {
		setUpstream := strings.TrimSpace(session.PRURL) == ""
		if err := r.pushBranch(ctx, run.WorktreePath, session.Branch, setUpstream); err != nil {
			return fail("push", err)
		}
		if session.AutoPR && strings.TrimSpace(session.PRURL) == "" {
			prURL, err := r.createDraftPR(ctx, run.WorktreePath, opts.BaseBranch, session.Branch, opts.Prompt, session.Tool, session.ID)
			if err != nil {
				return fail("create-pr", err)
			}
			if err := r.state.SetSessionPRURL(session.ID, prURL); err != nil {
				return fail("store-pr", err)
			}
			session.PRURL = prURL
			_ = r.state.AppendRunEvent(state.RunEvent{
				RunID:   run.ID,
				Type:    "pr",
				Message: "Draft PR created: " + prURL,
			})
		}
	}

	if err := r.state.CompleteRun(run.ID, string(task.StateCompleted), commitSHA, commitMsg, ""); err != nil {
		return err
	}
	if err := r.updateSessionStatusIfLatest(session.ID, run.ID, string(task.StateCompleted)); err != nil {
		return err
	}
	_ = r.state.AppendRunEvent(state.RunEvent{
		RunID:   run.ID,
		Type:    "complete",
		Message: "Run completed",
	})
	return nil
}

func (r *Runner) runTool(ctx context.Context, toolName, workdir, prompt string) (string, error) {
	tool, err := ai.GetTool(toolName)
	if err != nil {
		return "", err
	}
	if !tool.IsAvailable() {
		return "", fmt.Errorf("AI tool %s not available", toolName)
	}

	result, err := tool.Execute(ctx, workdir, prompt)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("AI execution failed: %s", result.Output)
	}
	return strings.TrimSpace(result.Output), nil
}

func (r *Runner) runShell(ctx context.Context, workdir, cmdline string) error {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return nil
	}

	output, err := proc.Run(ctx, workdir, "sh", "-c", cmdline)
	if err != nil {
		return withOutput(err, output)
	}
	return nil
}

func (r *Runner) commitSessionChanges(ctx context.Context, toolName, workdir, prompt, commitMsg string) (sha, finalMsg string, changed bool, err error) {
	statusOut, err := proc.Run(ctx, workdir, "git", "status", "--porcelain")
	if err != nil {
		return "", "", false, fmt.Errorf("git status failed: %w", withOutput(err, statusOut))
	}
	if len(statusOut) == 0 {
		return "", "", false, nil
	}

	if output, err := proc.Run(ctx, workdir, "git", "add", "."); err != nil {
		return "", "", false, fmt.Errorf("git add failed: %w", withOutput(err, output))
	}

	finalMsg = strings.TrimSpace(commitMsg)
	if finalMsg == "" {
		generated, err := r.generateCommitMessage(ctx, toolName, workdir, prompt)
		if isCanceledError(err) {
			return "", "", false, err
		}
		if err != nil || strings.TrimSpace(generated) == "" {
			finalMsg = fallbackCommitMessage(prompt)
		} else {
			finalMsg = generated
		}
	}

	if output, err := proc.Run(ctx, workdir, "git", "commit", "-m", finalMsg); err != nil {
		return "", "", false, fmt.Errorf("git commit failed: %w", withOutput(err, output))
	}

	shaOut, err := proc.Run(ctx, workdir, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", "", false, fmt.Errorf("git rev-parse failed: %w", withOutput(err, shaOut))
	}

	return strings.TrimSpace(string(shaOut)), finalMsg, true, nil
}

func (r *Runner) generateCommitMessage(ctx context.Context, toolName, workdir, prompt string) (string, error) {
	summary, err := stagedDiffSummary(ctx, workdir)
	if err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "fog-commit-msg-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	commitPrompt := strings.TrimSpace(fmt.Sprintf(
		"Generate a git commit message for the staged changes.\n"+
			"Rules:\n"+
			"- Use Conventional Commits style.\n"+
			"- Return plain text only.\n"+
			"- First line <= 72 chars.\n"+
			"- Optional body allowed.\n"+
			"- Do not include code fences.\n\n"+
			"Task prompt:\n%s\n\n"+
			"Staged changes summary:\n%s\n",
		strings.TrimSpace(prompt),
		summary,
	))
	raw, err := r.runTool(ctx, toolName, tempDir, commitPrompt)
	if err != nil {
		return "", err
	}

	msg := normalizeCommitMessage(raw)
	if msg == "" {
		return "", fmt.Errorf("empty commit message generated")
	}
	return msg, nil
}

func stagedDiffSummary(ctx context.Context, workdir string) (string, error) {
	nameStatusOut, err := proc.Run(ctx, workdir, "git", "diff", "--cached", "--name-status")
	if err != nil {
		return "", fmt.Errorf("git diff --name-status failed: %w", withOutput(err, nameStatusOut))
	}

	statOut, err := proc.Run(ctx, workdir, "git", "diff", "--cached", "--stat")
	if err != nil {
		return "", fmt.Errorf("git diff --stat failed: %w", withOutput(err, statOut))
	}

	patchOut, err := proc.Run(ctx, workdir, "git", "diff", "--cached", "--no-color")
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", withOutput(err, patchOut))
	}

	trimmedPatch := truncate(strings.TrimSpace(string(patchOut)), 12000)
	return strings.TrimSpace(
		"Name status:\n" + strings.TrimSpace(string(nameStatusOut)) +
			"\n\nStat:\n" + strings.TrimSpace(string(statOut)) +
			"\n\nPatch (truncated):\n" + trimmedPatch,
	), nil
}

func normalizeCommitMessage(raw string) string {
	msg := strings.TrimSpace(raw)
	if strings.HasPrefix(msg, "```") {
		msg = strings.TrimPrefix(msg, "```")
		msg = strings.TrimSpace(msg)
		if idx := strings.LastIndex(msg, "```"); idx >= 0 {
			msg = strings.TrimSpace(msg[:idx])
		}
		msg = strings.TrimPrefix(msg, "git")
		msg = strings.TrimPrefix(msg, "commit")
		msg = strings.TrimSpace(msg)
	}
	return truncate(msg, 5000)
}

func fallbackCommitMessage(prompt string) string {
	base := strings.TrimSpace(prompt)
	if base == "" {
		base = "update code"
	}
	return fmt.Sprintf("feat: %s\n\nGenerated by Fog session", truncate(base, 120))
}

func (r *Runner) pushBranch(ctx context.Context, workdir, branch string, setUpstream bool) error {
	args := []string{"push", "origin", branch}
	if setUpstream {
		args = []string{"push", "-u", "origin", branch}
	}
	argv := append([]string{"git"}, args...)
	output, err := proc.Run(ctx, workdir, argv[0], argv[1:]...)
	if err != nil {
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), withOutput(err, output))
	}
	return nil
}

func (r *Runner) createDraftPR(ctx context.Context, workdir, baseBranch, branch, prompt, tool, sessionID string) (string, error) {
	if !commandExists("gh") {
		return "", fmt.Errorf("gh CLI not available")
	}
	title := fmt.Sprintf("feat: %s", strings.TrimSpace(prompt))
	body := fmt.Sprintf("Generated by Fog session\n\nSession ID: %s\nAI Tool: %s\n\nPrompt:\n%s",
		sessionID,
		tool,
		strings.TrimSpace(prompt),
	)
	output, err := proc.Run(
		ctx,
		workdir,
		"gh", "pr", "create",
		"--draft",
		"--base", baseBranch,
		"--head", branch,
		"--title", title,
		"--body", body,
	)
	if err != nil {
		return "", fmt.Errorf("create draft PR failed: %w", withOutput(err, output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *Runner) setRunPhase(sessionID, runID, phase string) error {
	if err := r.state.SetRunState(runID, phase); err != nil {
		return err
	}
	return r.updateSessionStatusIfLatest(sessionID, runID, phase)
}

func (r *Runner) updateSessionStatusIfLatest(sessionID, runID, status string) error {
	latest, found, err := r.state.GetLatestRun(sessionID)
	if err != nil {
		return err
	}
	if !found || latest.ID != strings.TrimSpace(runID) {
		return nil
	}
	return r.state.UpdateSessionStatus(sessionID, status)
}

func (r *Runner) registerActiveRun(sessionID, runID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[sessionID] = &activeRun{
		sessionID: sessionID,
		runID:     runID,
		cancel:    cancel,
	}
}

func (r *Runner) clearActiveRun(sessionID, runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.active[sessionID]
	if !ok || current == nil {
		return
	}
	if strings.TrimSpace(current.runID) != strings.TrimSpace(runID) {
		return
	}
	delete(r.active, sessionID)
}

func (r *Runner) detachWorktreeHead(worktreePath string) error {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return nil
	}
	if _, err := os.Stat(worktreePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("check worktree path %s: %w", worktreePath, err)
	}

	output, err := proc.Run(context.Background(), worktreePath, "git", "checkout", "--detach")
	if err != nil {
		return fmt.Errorf("detach worktree %s: %w", worktreePath, withOutput(err, output))
	}
	return nil
}

func withOutput(err error, output []byte) error {
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, text)
}

func isCanceledError(err error) bool {
	return errors.Is(err, proc.ErrCanceled) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func runWorktreeName(branch, runID string) string {
	branch = nonWorktreeNameChar.ReplaceAllString(strings.TrimSpace(branch), "-")
	branch = strings.Trim(branch, "-._")
	if branch == "" {
		branch = "run"
	}
	if len(branch) > 180 {
		branch = branch[:180]
		branch = strings.Trim(branch, "-._")
		if branch == "" {
			branch = "run"
		}
	}
	suffix := strings.TrimSpace(runID)
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	suffix = nonWorktreeNameChar.ReplaceAllString(suffix, "")
	if suffix == "" {
		suffix = "latest"
	}
	return branch + "-" + suffix
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
