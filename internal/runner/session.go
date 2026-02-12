package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
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

	worktreePath, err := r.createWorktreePath(opts.RepoPath, opts.Branch)
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
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Prompt:    opts.Prompt,
		State:     string(task.StateCreated),
		CreatedAt: now,
		UpdatedAt: now,
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

	now := time.Now().UTC()
	run := state.Run{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Prompt:    prompt,
		State:     string(task.StateCreated),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.state.CreateRun(run); err != nil {
		_ = r.state.SetSessionBusy(session.ID, false)
		return state.Session{}, state.Run{}, sessionRunOptions{}, err
	}

	return session, run, sessionRunOptions{
		Prompt:     prompt,
		BaseBranch: "main",
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
	defer func() {
		if err := r.state.SetSessionBusy(session.ID, false); err != nil && retErr == nil {
			retErr = err
		}
	}()

	fail := func(phase string, err error) error {
		_ = r.state.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "error",
			Message: phase + ": " + err.Error(),
		})
		_ = r.state.CompleteRun(run.ID, string(task.StateFailed), "", "", err.Error())
		_ = r.state.UpdateSessionStatus(session.ID, string(task.StateFailed))
		return err
	}

	if opts.SetupCmd != "" {
		if err := r.state.SetRunState(run.ID, string(task.StateSetup)); err != nil {
			return err
		}
		if err := r.state.UpdateSessionStatus(session.ID, string(task.StateSetup)); err != nil {
			return err
		}
		_ = r.state.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "setup",
			Message: "Running setup command",
		})
		if err := r.runShell(session.WorktreePath, opts.SetupCmd); err != nil {
			return fail("setup", err)
		}
	}

	if err := r.state.SetRunState(run.ID, string(task.StateAIRunning)); err != nil {
		return err
	}
	if err := r.state.UpdateSessionStatus(session.ID, string(task.StateAIRunning)); err != nil {
		return err
	}
	_ = r.state.AppendRunEvent(state.RunEvent{
		RunID:   run.ID,
		Type:    "ai_start",
		Message: "Running AI tool",
	})
	aiOutput, err := r.runTool(session.Tool, session.WorktreePath, opts.Prompt)
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
		if err := r.state.SetRunState(run.ID, string(task.StateValidating)); err != nil {
			return err
		}
		if err := r.state.UpdateSessionStatus(session.ID, string(task.StateValidating)); err != nil {
			return err
		}
		if err := r.runShell(session.WorktreePath, opts.ValidateCmd); err != nil {
			return fail("validate", err)
		}
	}

	if err := r.state.SetRunState(run.ID, string(task.StateCommitted)); err != nil {
		return err
	}
	if err := r.state.UpdateSessionStatus(session.ID, string(task.StateCommitted)); err != nil {
		return err
	}
	commitSHA, commitMsg, changed, err := r.commitSessionChanges(session.Tool, session.WorktreePath, opts.Prompt, opts.CommitMsg)
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
		if err := r.pushBranch(session.WorktreePath, session.Branch, setUpstream); err != nil {
			return fail("push", err)
		}
		if session.AutoPR && strings.TrimSpace(session.PRURL) == "" {
			prURL, err := r.createDraftPR(session.WorktreePath, opts.BaseBranch, session.Branch, opts.Prompt, session.Tool, session.ID)
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
	if err := r.state.UpdateSessionStatus(session.ID, string(task.StateCompleted)); err != nil {
		return err
	}
	_ = r.state.AppendRunEvent(state.RunEvent{
		RunID:   run.ID,
		Type:    "complete",
		Message: "Run completed",
	})
	return nil
}

func (r *Runner) runTool(toolName, workdir, prompt string) (string, error) {
	tool, err := ai.GetTool(toolName)
	if err != nil {
		return "", err
	}
	if !tool.IsAvailable() {
		return "", fmt.Errorf("AI tool %s not available", toolName)
	}

	result, err := tool.Execute(workdir, prompt)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("AI execution failed: %s", result.Output)
	}
	return strings.TrimSpace(result.Output), nil
}

func (r *Runner) runShell(workdir, cmdline string) error {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return nil
	}

	cmd := exec.Command("sh", "-c", cmdline)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", err.Error(), strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *Runner) commitSessionChanges(toolName, workdir, prompt, commitMsg string) (sha, finalMsg string, changed bool, err error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workdir
	statusOut, err := cmd.Output()
	if err != nil {
		return "", "", false, fmt.Errorf("git status failed: %w", err)
	}
	if len(statusOut) == 0 {
		return "", "", false, nil
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = workdir
	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", "", false, fmt.Errorf("git add failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	finalMsg = strings.TrimSpace(commitMsg)
	if finalMsg == "" {
		generated, err := r.generateCommitMessage(toolName, workdir, prompt)
		if err != nil || strings.TrimSpace(generated) == "" {
			finalMsg = fallbackCommitMessage(prompt)
		} else {
			finalMsg = generated
		}
	}

	commitCmd := exec.Command("git", "commit", "-m", finalMsg)
	commitCmd.Dir = workdir
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return "", "", false, fmt.Errorf("git commit failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	shaCmd := exec.Command("git", "rev-parse", "HEAD")
	shaCmd.Dir = workdir
	shaOut, err := shaCmd.Output()
	if err != nil {
		return "", "", false, fmt.Errorf("git rev-parse failed: %w", err)
	}

	return strings.TrimSpace(string(shaOut)), finalMsg, true, nil
}

func (r *Runner) generateCommitMessage(toolName, workdir, prompt string) (string, error) {
	summary, err := stagedDiffSummary(workdir)
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
	raw, err := r.runTool(toolName, tempDir, commitPrompt)
	if err != nil {
		return "", err
	}

	msg := normalizeCommitMessage(raw)
	if msg == "" {
		return "", fmt.Errorf("empty commit message generated")
	}
	return msg, nil
}

func stagedDiffSummary(workdir string) (string, error) {
	nameStatusCmd := exec.Command("git", "diff", "--cached", "--name-status")
	nameStatusCmd.Dir = workdir
	nameStatusOut, err := nameStatusCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff --name-status failed: %w\n%s", err, strings.TrimSpace(string(nameStatusOut)))
	}

	statCmd := exec.Command("git", "diff", "--cached", "--stat")
	statCmd.Dir = workdir
	statOut, err := statCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff --stat failed: %w\n%s", err, strings.TrimSpace(string(statOut)))
	}

	patchCmd := exec.Command("git", "diff", "--cached", "--no-color")
	patchCmd.Dir = workdir
	patchOut, err := patchCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\n%s", err, strings.TrimSpace(string(patchOut)))
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

func (r *Runner) pushBranch(workdir, branch string, setUpstream bool) error {
	args := []string{"push", "origin", branch}
	if setUpstream {
		args = []string{"push", "-u", "origin", branch}
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *Runner) createDraftPR(workdir, baseBranch, branch, prompt, tool, sessionID string) (string, error) {
	if !commandExists("gh") {
		return "", fmt.Errorf("gh CLI not available")
	}
	title := fmt.Sprintf("feat: %s", strings.TrimSpace(prompt))
	body := fmt.Sprintf("Generated by Fog session\n\nSession ID: %s\nAI Tool: %s\n\nPrompt:\n%s",
		sessionID,
		tool,
		strings.TrimSpace(prompt),
	)
	cmd := exec.Command(
		"gh", "pr", "create",
		"--draft",
		"--base", baseBranch,
		"--head", branch,
		"--title", title,
		"--body", body,
	)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create draft PR failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
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
