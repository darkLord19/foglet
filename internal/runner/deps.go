package runner

import (
	"github.com/darkLord19/foglet/internal/ai"
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

// RunStore is the session and run state a run reads and writes as it executes.
//
// It is narrow by intent: repos, settings, secrets and task state are not part of
// it, even though *state.Store provides them. The reads that are included earn
// their place — GetLatestRun gates whether a run may still write session status,
// and ListRuns/ListRunEvents recover the tool conversation id that makes a
// follow-up resume rather than restart.
type RunStore interface {
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
)
