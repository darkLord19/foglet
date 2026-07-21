package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/darkLord19/foglet/internal/state"
)

// These tests drive executeSessionRun end to end through the RunStore, Settings
// and ToolFactory seams. Before those seams existed, nothing past the agent call
// was reachable: ai.GetTool hard-failed without an installed CLI.
//
// The commit phase still shells out to git, so tests that run past it use a real
// temporary repository. Candidate 4 (one git module over internal/proc) is what
// removes that remaining dependency.

func initTestWorktree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}

// writeFile creates a change for the commit phase to pick up.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func testSession(worktree string) state.Session {
	return state.Session{
		ID:           "session-1",
		RepoName:     "acme/api",
		Branch:       "fog/test",
		WorktreePath: worktree,
		Tool:         "claude",
		Model:        "sonnet",
		Status:       "CREATED",
		Busy:         true,
	}
}

func testRun(worktree string) state.Run {
	return state.Run{
		ID:           "run-1",
		SessionID:    "session-1",
		Prompt:       "add a feature",
		WorktreePath: worktree,
		State:        "CREATED",
	}
}

// ---------------------------------------------------------------------------
// The busy-flag leak. These are the regression tests for the three early
// returns that used to exit before the cleanup defer was installed, leaving the
// session permanently wedged because follow-ups reject a busy session.
// ---------------------------------------------------------------------------

func TestExecuteSessionRunClearsBusyOnMissingBaseBranch(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	r := newTestRunner(store, &fakeTool{name: "claude", available: true}, nil)

	err := r.executeSessionRun(testSession("/tmp/wt"), testRun("/tmp/wt"), sessionRunOptions{
		Prompt: "add a feature",
		// BaseBranch deliberately empty.
	})
	if err == nil {
		t.Fatal("expected an error for a missing base branch")
	}
	if !store.busyCleared() {
		t.Fatal("session left busy: a follow-up would be rejected forever")
	}
}

func TestExecuteSessionRunClearsBusyOnMissingWorktreePath(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	r := newTestRunner(store, &fakeTool{name: "claude", available: true}, nil)

	session := testSession("")
	run := testRun("")

	err := r.executeSessionRun(session, run, sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatal("expected an error for a missing worktree path")
	}
	if !store.busyCleared() {
		t.Fatal("session left busy: a follow-up would be rejected forever")
	}
}

func TestExecuteSessionRunClearsBusyOnAgentFailure(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{name: "claude", available: true, err: errors.New("boom")}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	}); err == nil {
		t.Fatal("expected the agent failure to propagate")
	}
	if !store.busyCleared() {
		t.Fatal("session left busy after an agent failure")
	}
}

func TestExecuteSessionRunWithoutStoreDoesNotPanic(t *testing.T) {
	r := New(nil) // no state store configured
	err := r.executeSessionRun(testSession("/tmp/wt"), testRun("/tmp/wt"), sessionRunOptions{
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatal("expected an error when no store is configured")
	}
}

// ---------------------------------------------------------------------------
// Phase sequencing and event emission.
// ---------------------------------------------------------------------------

func TestExecuteSessionRunRecordsPhasesInOrder(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{name: "claude", available: true, output: "done"}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	writeFile(t, wt, "feature.txt", "work")

	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:      "add a feature",
		BaseBranch:  "main",
		SetupCmd:    "true",
		Validate:    true,
		ValidateCmd: "true",
		CommitMsg:   "feat: add a feature",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	want := []string{"SETUP", "AI_RUNNING", "VALIDATING", "COMMITTED", "COMPLETED"}
	if got := store.runStates; !equalStrings(got, want) {
		t.Errorf("phase transcript = %v, want %v", got, want)
	}
}

func TestExecuteSessionRunSkipsOptionalPhases(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	r := newTestRunner(store, &fakeTool{name: "claude", available: true, output: "done"}, nil)

	wt := initTestWorktree(t)
	writeFile(t, wt, "feature.txt", "work")

	// No SetupCmd, and Validate off: those phases must not be recorded.
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		CommitMsg:  "feat: add a feature",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	want := []string{"AI_RUNNING", "COMMITTED", "COMPLETED"}
	if got := store.runStates; !equalStrings(got, want) {
		t.Errorf("phase transcript = %v, want %v", got, want)
	}
}

func TestExecuteSessionRunEmitsEventsInOrder(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{
		name:           "claude",
		available:      true,
		output:         "all done",
		conversationID: "conv-42",
	}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	writeFile(t, wt, "feature.txt", "work")

	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		SetupCmd:   "true",
		CommitMsg:  "feat: add a feature",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	want := []string{"setup", "ai_start", "ai_session", "ai_output", "complete"}
	if got := store.eventTypes(); !equalStrings(got, want) {
		t.Errorf("event transcript = %v, want %v", got, want)
	}
}

func TestExecuteSessionRunPersistsConversationID(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{name: "claude", available: true, output: "ok", conversationID: "conv-42"}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		CommitMsg:  "feat: x",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	// AGENTS.md: a follow-up reuses the tool conversation id, which is stored as
	// an ai_session run event.
	event, found := store.eventOfType("ai_session")
	if !found {
		t.Fatal("no ai_session event recorded")
	}
	if event.Data != "conv-42" {
		t.Errorf("ai_session data = %q, want %q", event.Data, "conv-42")
	}
}

func TestExecuteSessionRunResumesPriorConversation(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	// A previous run in the same session recorded a conversation id.
	store.runs["run-0"] = &state.Run{ID: "run-0", SessionID: "session-1"}
	_ = store.AppendRunEvent(state.RunEvent{RunID: "run-0", Type: "ai_session", Data: "conv-earlier"})

	tool := &fakeTool{name: "claude", available: true, output: "ok"}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "follow up",
		BaseBranch: "main",
		CommitMsg:  "feat: x",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	if got := tool.request().ConversationID; got != "conv-earlier" {
		t.Errorf("resumed conversation id = %q, want %q", got, "conv-earlier")
	}
}

func TestExecuteSessionRunPassesPromptAndModelToAgent(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{name: "claude", available: true, output: "ok"}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		CommitMsg:  "feat: x",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	req := tool.request()
	if !strings.HasPrefix(req.Prompt, "add a feature") {
		t.Errorf("agent prompt = %q, want it to start with the user prompt", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "<commit_message>") {
		t.Error("agent prompt is missing the commit-message instructions")
	}
	if req.Model != "sonnet" {
		t.Errorf("agent model = %q, want %q", req.Model, "sonnet")
	}
	if req.Workdir != wt {
		t.Errorf("agent workdir = %q, want the run worktree %q", req.Workdir, wt)
	}
}

// ---------------------------------------------------------------------------
// Failure and cancellation.
// ---------------------------------------------------------------------------

func TestExecuteSessionRunMarksAgentFailureFailed(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{name: "claude", available: true, output: "partial", err: errors.New("agent exploded")}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	}); err == nil {
		t.Fatal("expected an error")
	}

	if got := lastString(store.runStates); got != "FAILED" {
		t.Errorf("terminal run state = %q, want FAILED", got)
	}
	if got := lastString(store.sessionStates); got != "FAILED" {
		t.Errorf("terminal session status = %q, want FAILED", got)
	}
	// Output produced before the failure must be preserved for debugging.
	if _, found := store.eventOfType("ai_output"); !found {
		t.Error("agent output was discarded on failure")
	}
	if _, found := store.eventOfType("error"); !found {
		t.Error("no error event recorded")
	}
}

func TestExecuteSessionRunMarksCancellationCancelled(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")

	// The agent blocks until the run's context is cancelled.
	tool := &fakeTool{
		name:      "claude",
		available: true,
		block: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	done := make(chan error, 1)
	go func() {
		done <- r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
			Prompt:     "add a feature",
			BaseBranch: "main",
		})
	}()

	// Cancel via the runner's own registry, which is what CancelSessionLatestRun does.
	cancel := waitForActiveRun(t, r, "session-1")
	cancel()

	if err := <-done; err == nil {
		t.Fatal("expected a cancellation error")
	}

	if got := lastString(store.runStates); got != "CANCELLED" {
		t.Errorf("terminal run state = %q, want CANCELLED", got)
	}
	if _, found := store.eventOfType("cancelled"); !found {
		t.Error("no cancelled event recorded")
	}
	if !store.busyCleared() {
		t.Error("session left busy after cancellation")
	}
}

func TestExecuteSessionRunFailsWhenToolUnavailable(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{name: "claude", available: false}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatal("expected an error when the tool is unavailable")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("error = %v, want it to mention availability", err)
	}
	if !store.busyCleared() {
		t.Error("session left busy when the tool was unavailable")
	}
}

func TestExecuteSessionRunFailsWhenToolUnknown(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	r := newTestRunner(store, nil, nil) // factory reports unknown tool

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	}); err == nil {
		t.Fatal("expected an error for an unknown tool")
	}
	if !store.busyCleared() {
		t.Error("session left busy for an unknown tool")
	}
}

// ---------------------------------------------------------------------------
// Commit behaviour.
// ---------------------------------------------------------------------------

func TestExecuteSessionRunReportsNoChangesToCommit(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	r := newTestRunner(store, &fakeTool{name: "claude", available: true, output: "nothing to do"}, nil)

	// Clean worktree: the agent changed nothing.
	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	event, found := store.eventOfType("commit")
	if !found {
		t.Fatal("no commit event recorded")
	}
	if !strings.Contains(event.Message, "No changes") {
		t.Errorf("commit event = %q, want it to report no changes", event.Message)
	}
	if got := lastString(store.runStates); got != "COMPLETED" {
		t.Errorf("terminal state = %q, want COMPLETED — an empty run still completes", got)
	}
}

func TestExecuteSessionRunUsesCommitMessageFromAgentOutput(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{
		name:      "claude",
		available: true,
		output:    "I did the work.\n<commit_message>feat: extracted from output</commit_message>",
	}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	writeFile(t, wt, "feature.txt", "work")

	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		// No CommitMsg: it must be extracted from the agent's output.
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	if got := gitLastCommitMessage(t, wt); got != "feat: extracted from output" {
		t.Errorf("commit message = %q, want it extracted from agent output", got)
	}
}

func TestExecuteSessionRunGeneratesCommitMessageWhenAgentOmitsOne(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	// No <commit_message> tag in the output, and no CommitMsg option: the
	// pipeline must ask the agent for one in a second call.
	tool := &fakeTool{name: "claude", available: true, output: "feat: generated by the agent"}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	writeFile(t, wt, "feature.txt", "work")

	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	if tool.calls < 2 {
		t.Errorf("agent called %d times, want a second call for the commit message", tool.calls)
	}
	if got := gitLastCommitMessage(t, wt); got != "feat: generated by the agent" {
		t.Errorf("commit message = %q, want the generated one", got)
	}
}

func TestExecuteSessionRunPersistsStreamedChunks(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	tool := &fakeTool{
		name:      "claude",
		available: true,
		output:    "done",
		chunks:    []string{"thinking...", " still thinking...", " done"},
	}
	r := newTestRunner(store, tool, nil)

	wt := initTestWorktree(t)
	if err := r.executeSessionRun(testSession(wt), testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		CommitMsg:  "feat: x",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	event, found := store.eventOfType("ai_stream")
	if !found {
		t.Fatal("streamed chunks were not persisted as an ai_stream event")
	}
	if !strings.Contains(event.Data, "still thinking") {
		t.Errorf("ai_stream data = %q, want the buffered chunks", event.Data)
	}
}

func TestExecuteSessionRunDoesNotPushWithoutAutoPR(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	r := newTestRunner(store, &fakeTool{name: "claude", available: true, output: "ok"}, nil)

	// The worktree has no remote, so any push attempt would fail the run.
	wt := initTestWorktree(t)
	writeFile(t, wt, "feature.txt", "work")

	session := testSession(wt)
	session.AutoPR = false
	session.PRURL = ""

	if err := r.executeSessionRun(session, testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		CommitMsg:  "feat: x",
	}); err != nil {
		t.Fatalf("executeSessionRun pushed without AutoPR: %v", err)
	}
	if len(store.prURLs) != 0 {
		t.Errorf("PR URL written without AutoPR: %v", store.prURLs)
	}
}

// ---------------------------------------------------------------------------
// Settings reach the pipeline through the SettingsReader seam.
// ---------------------------------------------------------------------------

func TestNotificationsRespectSetting(t *testing.T) {
	r := newTestRunner(newFakeRunStore(), nil, fakeSettings{"default_notify": "true"})
	if !r.notificationsEnabled() {
		t.Error("notifications should be enabled")
	}

	r = newTestRunner(newFakeRunStore(), nil, fakeSettings{"default_notify": "false"})
	if r.notificationsEnabled() {
		t.Error("notifications should be disabled")
	}

	r = newTestRunner(newFakeRunStore(), nil, fakeSettings{})
	if r.notificationsEnabled() {
		t.Error("notifications should default to disabled")
	}
}

func TestKeepAwakeRespectsSetting(t *testing.T) {
	r := newTestRunner(newFakeRunStore(), nil, fakeSettings{"keep_awake": "true"})
	if !r.keepAwakeEnabled() {
		t.Error("keep-awake should be enabled")
	}

	r = newTestRunner(newFakeRunStore(), nil, fakeSettings{})
	if r.keepAwakeEnabled() {
		t.Error("keep-awake should default to disabled")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// waitForActiveRun blocks until the runner has registered an active run for the
// session, then returns its cancel func.
func waitForActiveRun(t *testing.T, r *Runner, sessionID string) context.CancelFunc {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		active := r.active[sessionID]
		r.mu.Unlock()
		if active != nil && active.cancel != nil {
			return active.cancel
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no active run registered for session %q", sessionID)
	return nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func lastString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func gitLastCommitMessage(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// ── publish phase ───────────────────────────────────────────────────────────
//
// Until the Publisher seam existed these were unreachable: opening a PR needed
// a gh binary, a git remote and a network.

// initTestWorktreeWithRemote gives the worktree a local bare remote so a push
// succeeds without a network.
func initTestWorktreeWithRemote(t *testing.T) string {
	t.Helper()
	remote := t.TempDir()
	bare := exec.Command("git", "init", "--bare", remote)
	if out, err := bare.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	dir := initTestWorktree(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("remote", "add", "origin", remote)
	run("branch", "-M", "fog/test")
	return dir
}

func TestExecuteSessionRunOpensDraftPRWhenAutoPRSet(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	pub := &fakePublisher{available: true, url: "https://example.invalid/pr/7"}
	r := newTestRunnerWithPublisher(store, &fakeTool{name: "claude", available: true, output: "ok"}, nil, pub)

	wt := initTestWorktreeWithRemote(t)
	writeFile(t, wt, "feature.txt", "work")

	session := testSession(wt)
	session.AutoPR = true

	if err := r.executeSessionRun(session, testRun(wt), sessionRunOptions{
		Prompt:     "add a feature",
		BaseBranch: "main",
		CommitMsg:  "feat: x",
		PRTitle:    "Add a feature",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}

	if pub.calls != 1 {
		t.Fatalf("publisher called %d times, want 1", pub.calls)
	}
	if !pub.gotDraft {
		t.Error("PR was not opened as a draft")
	}
	if pub.gotBase != "main" || pub.gotBranch != "fog/test" {
		t.Errorf("PR base/branch = %q/%q, want main/fog/test", pub.gotBase, pub.gotBranch)
	}
	if pub.gotTitle != "Add a feature" {
		t.Errorf("PR title = %q, want the supplied one", pub.gotTitle)
	}
	if len(store.prURLs) != 1 || store.prURLs[0] != "https://example.invalid/pr/7" {
		t.Errorf("PR URL not persisted: %v", store.prURLs)
	}
	if event, found := store.eventOfType("pr"); !found || !strings.Contains(event.Message, "pr/7") {
		t.Errorf("no pr event recorded (found=%v, event=%+v)", found, event)
	}
}

func TestExecuteSessionRunFailsRunWhenPRCreationFails(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	pub := &fakePublisher{available: true, err: errors.New("gh exploded")}
	r := newTestRunnerWithPublisher(store, &fakeTool{name: "claude", available: true, output: "ok"}, nil, pub)

	wt := initTestWorktreeWithRemote(t)
	writeFile(t, wt, "feature.txt", "work")

	session := testSession(wt)
	session.AutoPR = true

	if err := r.executeSessionRun(session, testRun(wt), sessionRunOptions{
		Prompt: "add a feature", BaseBranch: "main", CommitMsg: "feat: x",
	}); err == nil {
		t.Fatal("expected the PR failure to fail the run")
	}
	if got := lastString(store.runStates); got != "FAILED" {
		t.Errorf("terminal state = %q, want FAILED", got)
	}
	if !store.busyCleared() {
		t.Error("session left busy after a PR failure")
	}
}

func TestExecuteSessionRunFailsWhenGhUnavailable(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	pub := &fakePublisher{available: false}
	r := newTestRunnerWithPublisher(store, &fakeTool{name: "claude", available: true, output: "ok"}, nil, pub)

	wt := initTestWorktreeWithRemote(t)
	writeFile(t, wt, "feature.txt", "work")

	session := testSession(wt)
	session.AutoPR = true

	err := r.executeSessionRun(session, testRun(wt), sessionRunOptions{
		Prompt: "add a feature", BaseBranch: "main", CommitMsg: "feat: x",
	})
	if err == nil {
		t.Fatal("expected an error when gh is unavailable")
	}
	if pub.calls != 0 {
		t.Error("publisher was called despite reporting unavailable")
	}
}

// An existing PR means the branch is pushed but no second PR is opened.
func TestExecuteSessionRunPushesWithoutReopeningExistingPR(t *testing.T) {
	store := newFakeRunStore()
	store.seed("session-1", "run-1")
	pub := &fakePublisher{available: true, url: "https://example.invalid/pr/9"}
	r := newTestRunnerWithPublisher(store, &fakeTool{name: "claude", available: true, output: "ok"}, nil, pub)

	wt := initTestWorktreeWithRemote(t)
	writeFile(t, wt, "feature.txt", "work")

	session := testSession(wt)
	session.AutoPR = true
	session.PRURL = "https://example.invalid/pr/1" // already open

	if err := r.executeSessionRun(session, testRun(wt), sessionRunOptions{
		Prompt: "add a feature", BaseBranch: "main", CommitMsg: "feat: x",
	}); err != nil {
		t.Fatalf("executeSessionRun: %v", err)
	}
	if pub.calls != 0 {
		t.Errorf("publisher called %d times, want 0 — a PR already exists", pub.calls)
	}
	if len(store.prURLs) != 0 {
		t.Errorf("PR URL rewritten: %v", store.prURLs)
	}
}
