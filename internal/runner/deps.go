package runner

import (
	"context"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/state"
)

// The seams the run pipeline depends on.
//
// These are declared by the runner (the consumer), not by the packages that
// satisfy them. *state.Store implements RunStore and SettingsReader with its
// existing method set, and ai.GetTool is a ToolFactory, so production wiring is
// unchanged — the seams exist so tests can substitute fakes and drive a run to
// completion without SQLite, a git repository, or an installed agent CLI.
//
// See CONTEXT.md for why the pipeline's phases are deliberately *not* a seam.

// RunStore is the session and run state the runner reads and writes.
//
// Narrow by intent: 14 methods against *state.Store's 43. Repos, settings,
// secrets and task state are reached through their own seams or not at all.
//
// This is the runner's only route to session and run persistence — the Runner
// holds no *state.Store — so a Runner built with fakes behaves like a real one
// rather than silently doing nothing.
type RunStore interface {
	CreateSession(session state.Session) error
	GetSession(id string) (state.Session, bool, error)
	ListSessions() ([]state.Session, error)
	CreateRun(run state.Run) error
	GetRun(id string) (state.Run, bool, error)
	SetRunState(id, state string) error
	CompleteRun(id, state, commitSHA, commitMsg, runErr string) error
	AppendRunEvent(event state.RunEvent) error
	ListRuns(sessionID string) ([]state.Run, error)
	ListRunEvents(runID string, limit int) ([]state.RunEvent, error)
	GetLatestRun(sessionID string) (state.Run, bool, error)
	UpdateSessionStatus(id, status string) error
	SetSessionBusy(id string, busy bool) error
	SetSessionPRURL(id, prURL string) error
}

// SettingsReader reads user preferences that alter how a run behaves.
//
// GetDefaultTool belongs here rather than as a raw GetSetting key because
// toolcfg.ResolveTool consumes it as its own interface, and the default tool has
// resolution rules of its own.
type SettingsReader interface {
	GetSetting(key string) (value string, found bool, err error)
	GetDefaultTool() (tool string, found bool, err error)
}

// RepoReader resolves a managed repo by name.
type RepoReader interface {
	GetRepoByName(name string) (state.Repo, bool, error)
}

// Publisher opens a pull request for a session branch.
//
// Fog shells out to the gh CLI for this. It is the last dependency in the run
// pipeline that a test cannot satisfy — it needs a gh binary, a git remote and
// a network — so the push/PR branch of executeSessionRun was unreachable even
// after the other seams landed.
//
// Available is separate from CreatePR because the pipeline checks availability
// first and reports a distinct error; folding them together would make "gh is
// not installed" and "opening the PR failed" indistinguishable.
type Publisher interface {
	Available() bool
	CreatePR(ctx context.Context, workdir, title, body, baseBranch, branch string, draft bool) (string, error)
}

// ghPublisher is the production Publisher, backed by the gh CLI.
type ghPublisher struct{}

func (ghPublisher) Available() bool { return ghcli.IsGhAvailable() }

func (ghPublisher) CreatePR(ctx context.Context, workdir, title, body, baseBranch, branch string, draft bool) (string, error) {
	return ghcli.CreatePRWithContext(ctx, workdir, title, body, baseBranch, branch, draft)
}

// ToolFactory resolves a canonical tool name to an AI tool adapter.
//
// ai.GetTool is the production factory. It reports an error for unknown names and
// the adapter it returns reports its own availability, so a run can fail cleanly
// before spawning anything.
type ToolFactory func(name string) (ai.Tool, error)

// Compile-time proof that the production adapters satisfy the seams.
var (
	_ RunStore       = (*state.Store)(nil)
	_ SettingsReader = (*state.Store)(nil)
	_ RepoReader     = (*state.Store)(nil)
	_ ToolFactory    = ai.GetTool
	_ Publisher      = ghPublisher{}
)
