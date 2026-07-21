# Context — Domain Language

Fog's established vocabulary lives in [AGENTS.md](./AGENTS.md): **Session**, **Run**, **Run Event**,
**Worktree**, **Host Guard**, **AI Tool Adapter**. The domain rules for the board — **Task**,
**Status**, **Origin**, **Provider**, **WorkKind** — are defined in the package doc of
`internal/task/task.go`, which is the authority for those terms.

This file records terms introduced by architectural work that are **not** derivable from either.

## Run Store

The seam a run reads and writes its own progress through: phase transitions, run events, terminal
state, and the session-level flags a run owns (busy, PR URL).

Defined by `internal/runner` as a consumer interface, not by `internal/state`. `*state.Store`
satisfies it in production with its existing method set; tests substitute a fake.

Narrow by intent: repos, settings, secrets and task state are excluded even though `*state.Store`
provides them (43 methods; the Run Store is 9). The reads it does include earn their place —
`GetLatestRun` gates whether a run may still write session status, and `ListRuns`/`ListRunEvents`
recover the tool conversation id that makes a follow-up resume rather than restart. `GetSession`,
`ListSessions` and the other pass-throughs are deliberately absent: faking them buys no leverage.

**Why it exists:** the run pipeline previously wrote directly to a concrete store, which made the
180-line execution core unreachable by tests. See `internal/runner/session_exec.go`.

**Hazard:** `Runner` holds both `state` and the seam fields. A `Runner` literal that sets only
`state` reads no settings and has no run store. Construct through `New`.

## Tool Factory

The seam that resolves a canonical tool name to an AI Tool Adapter.

Replaces the package-level `ai.GetTool` switch as the runner's dependency. `ai.GetTool` is the
production adapter; tests supply a factory returning a fake adapter.

**Why it exists:** `ai.GetTool` hard-fails when the named CLI is not installed, so with it wired in
directly no test could execute a run past the agent phase — commit, push, PR creation, and every
terminal state transition were unreachable.

## Phase

One named step of a run: setup, agent, validate, commit, publish.

Phases are **implementation, not interface** — they are straight-line code inside the run pipeline,
not injectable values. This is a deliberate choice: there is only one pipeline shape today (review
runs reuse the implementation pipeline as a follow-up), and one shape means one adapter, which would
make a phase seam indirection rather than a real seam. Revisit if a second shape appears — a
dry-run mode or a pipeline that skips commit would justify it.
