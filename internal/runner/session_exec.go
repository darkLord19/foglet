package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/util"
)

const commitMsgInstructions = `

IMPORTANT: Your response MUST end with a suggested git commit message for the changes you made, wrapped in <commit_message> tags.
Use Conventional Commits style (e.g., <commit_message>feat: add login functionality</commit_message>).
Do not include any other text inside the tags.
`

type activeRun struct {
	sessionID string
	runID     string
	cancel    context.CancelFunc
	// poweredOn records whether this run acquired the keep-awake assertion, so
	// clearActiveRun releases exactly what it acquired regardless of whether the
	// keep_awake setting is toggled while the run is in flight.
	poweredOn bool
}

type sessionRunOptions struct {
	Prompt      string
	SetupCmd    string
	Validate    bool
	ValidateCmd string
	BaseBranch  string
	CommitMsg   string
	PRTitle     string
}

func (r *Runner) executeSessionRun(session state.Session, run state.Run, opts sessionRunOptions) (retErr error) {
	if r.runs == nil {
		return errors.New("state store not configured")
	}

	// The caller marked the session busy before handing it over, so from here
	// every exit must release it. This defer is installed before the argument
	// checks below: returning from one of them without clearing the flag used to
	// wedge the session permanently, since follow-ups reject a busy session.
	defer func() {
		if err := r.runs.SetSessionBusy(session.ID, false); err != nil && retErr == nil {
			retErr = err
		}
	}()

	if strings.TrimSpace(opts.BaseBranch) == "" {
		return errors.New("base branch is required")
	}
	if strings.TrimSpace(run.WorktreePath) == "" {
		run.WorktreePath = strings.TrimSpace(session.WorktreePath)
	}
	if strings.TrimSpace(run.WorktreePath) == "" {
		return errors.New("run worktree path is required")
	}
	ctx, cancel := context.WithCancel(r.baseCtx)
	r.registerActiveRun(session.ID, run.ID, cancel)
	defer func() {
		r.clearActiveRun(session.ID, run.ID)
		cancel()
	}()

	fail := func(phase string, err error) error {
		terminalState := "FAILED"
		eventType := "error"
		message := phase + ": " + err.Error()
		if isCanceledError(err) {
			terminalState = "CANCELLED"
			eventType = "cancelled"
			message = phase + ": canceled"
		}
		_ = r.runs.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    eventType,
			Message: message,
		})
		_ = r.runs.CompleteRun(run.ID, terminalState, "", "", err.Error())
		_ = r.updateSessionStatusIfLatest(session.ID, run.ID, terminalState)
		if r.notificationsEnabled() {
			title := "Fog Session Failed"
			msg := fmt.Sprintf("Failed on %s (%s): %v", session.Branch, session.RepoName, err)
			if isCanceledError(err) {
				title = "Fog Session Cancelled"
				msg = fmt.Sprintf("Cancelled on %s (%s)", session.Branch, session.RepoName)
			}
			util.Notify(title, msg, session.ID)
		}
		return err
	}

	if opts.SetupCmd != "" {
		if err := r.setRunPhase(session.ID, run.ID, "SETUP"); err != nil {
			return err
		}
		_ = r.runs.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "setup",
			Message: "Running setup command",
		})
		if err := r.runShell(ctx, run.WorktreePath, opts.SetupCmd); err != nil {
			return fail("setup", err)
		}
	}

	if err := r.setRunPhase(session.ID, run.ID, "AI_RUNNING"); err != nil {
		return err
	}
	_ = r.runs.AppendRunEvent(state.RunEvent{
		RunID:   run.ID,
		Type:    "ai_start",
		Message: "Running AI tool",
	})
	streamWriter := newRunStreamWriter(r.runs, run.ID)
	conversationID := r.lookupConversationID(session.ID, run.ID)
	aiOutput, nextConversationID, err := r.runToolWithOptions(
		ctx,
		session.Tool,
		run.WorktreePath,
		opts.Prompt+commitMsgInstructions,
		session.Model,
		conversationID,
		streamWriter.Append,
	)
	streamWriter.Flush()
	if err != nil {
		if strings.TrimSpace(aiOutput) != "" {
			_ = r.runs.AppendRunEvent(state.RunEvent{
				RunID:   run.ID,
				Type:    "ai_output",
				Message: truncate(aiOutput, 8000),
			})
		}
		return fail("ai", err)
	}
	if nextConversationID != "" {
		_ = r.runs.AppendRunEvent(state.RunEvent{
			RunID: run.ID,
			Type:  "ai_session",
			Data:  nextConversationID,
		})
	}
	if strings.TrimSpace(aiOutput) != "" {
		_ = r.runs.AppendRunEvent(state.RunEvent{
			RunID:   run.ID,
			Type:    "ai_output",
			Message: truncate(aiOutput, 8000),
		})
	}

	if opts.Validate && opts.ValidateCmd != "" {
		if err := r.setRunPhase(session.ID, run.ID, "VALIDATING"); err != nil {
			return err
		}
		if err := r.runShell(ctx, run.WorktreePath, opts.ValidateCmd); err != nil {
			return fail("validate", err)
		}
	}

	if err := r.setRunPhase(session.ID, run.ID, "COMMITTED"); err != nil {
		return err
	}

	extractedMsg := opts.CommitMsg
	if extractedMsg == "" {
		extractedMsg = extractCommitMessage(aiOutput)
	}

	commitSHA, commitMsg, changed, err := r.commitSessionChanges(ctx, session.Tool, run.WorktreePath, opts.Prompt, extractedMsg)
	if err != nil {
		return fail("commit", err)
	}
	if !changed {
		_ = r.runs.AppendRunEvent(state.RunEvent{
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
			prURL, err := r.createDraftPR(ctx, run.WorktreePath, opts.BaseBranch, session.Branch, opts.Prompt, session.Tool, session.ID, opts.PRTitle)
			if err != nil {
				return fail("create-pr", err)
			}
			if err := r.runs.SetSessionPRURL(session.ID, prURL); err != nil {
				return fail("store-pr", err)
			}
			session.PRURL = prURL
			_ = r.runs.AppendRunEvent(state.RunEvent{
				RunID:   run.ID,
				Type:    "pr",
				Message: "Draft PR created: " + prURL,
			})
		}
	}

	if err := r.runs.CompleteRun(run.ID, "COMPLETED", commitSHA, commitMsg, ""); err != nil {
		return err
	}
	if err := r.updateSessionStatusIfLatest(session.ID, run.ID, "COMPLETED"); err != nil {
		return err
	}
	_ = r.runs.AppendRunEvent(state.RunEvent{
		RunID:   run.ID,
		Type:    "complete",
		Message: "Run completed",
	})
	if r.notificationsEnabled() {
		msg := fmt.Sprintf("Finished successfully on %s (%s)", session.Branch, session.RepoName)
		util.Notify("Fog Session Complete", msg, session.ID)
	}
	return nil
}

func (r *Runner) setRunPhase(sessionID, runID, phase string) error {
	if err := r.runs.SetRunState(runID, phase); err != nil {
		return err
	}
	return r.updateSessionStatusIfLatest(sessionID, runID, phase)
}

func (r *Runner) updateSessionStatusIfLatest(sessionID, runID, status string) error {
	latest, found, err := r.runs.GetLatestRun(sessionID)
	if err != nil {
		return err
	}
	if !found || latest.ID != strings.TrimSpace(runID) {
		return nil
	}
	return r.runs.UpdateSessionStatus(sessionID, status)
}

func (r *Runner) registerActiveRun(sessionID, runID string, cancel context.CancelFunc) {
	poweredOn := r.keepAwakeEnabled()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[sessionID] = &activeRun{
		sessionID: sessionID,
		runID:     runID,
		cancel:    cancel,
		poweredOn: poweredOn,
	}
	if poweredOn {
		r.power.Acquire()
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
	// Release based on what this run actually acquired, not the current setting,
	// so toggling keep_awake mid-run can never leak the power assertion.
	if current.poweredOn {
		r.power.Release()
	}
}
