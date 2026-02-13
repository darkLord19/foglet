# Fog (getfog.dev)

Fog turns your local machine into a "personal cloud" for AI coding agents.

This repo is intentionally local-first:
- Fog does not host an LLM.
- Fog runs existing AI coding CLIs (Cursor Agent, Claude Code, Gemini CLI, Aider) on the user's machine.
- Isolation and auditability come from Git worktrees and Git history.

## Product Principles

- Local-first: no mandatory cloud dependency.
- Explicit control: no silent edits to the user’s main checkout.
- Isolation by default: work happens in worktrees/branches, not the repo root.
- Auditability: state is persisted locally; changes are visible via Git.
- Security: secrets are encrypted at rest; no plaintext PAT storage.

## What Exists (And What Matters)

Fog has two execution surfaces:

- Sessions (desktop-first): long-lived branch/worktree conversations with follow-ups, forking, cancellation, and run-event streaming.
- Tasks (legacy/one-off): `fog run` and `/api/tasks/*` are a simpler workflow that still exists for CLI usage and backwards compatibility.

When implementing new UX/features, default to sessions unless explicitly asked to work on tasks.

## Components

- `cmd/wtx`: Git worktree manager (no AI, no networking).
- `cmd/fog`: CLI for one-off tasks + onboarding + managed repo registry.
- `cmd/fogd`: local daemon (HTTP API + session execution).
- `cmd/fogapp`: Wails desktop UI that talks to `fogd` and starts an embedded daemon when needed.
- `cmd/fogcloud`: optional distribution/control-plane (present in repo, not the current focus).

## Current Product Direction

Desktop-first sessions are the primary UX.

- A **session** is a long-lived branch + worktree "conversation" that can be followed up.
- A **run** is one execution inside a session (one prompt, one lifecycle).
- Follow-ups and re-runs operate on the same session worktree.
- Forking is explicit: it creates a new branch/worktree from the current session head.
- Streaming output is persisted as run events and can be consumed via SSE.

Slack/cloud code exists, but if you are adding features, default to desktop + local API unless explicitly asked to work on Slack/cloud.

## Architecture (Mental Model)

Data flow (desktop session):

1. `fogapp` talks to `fogd` at `http://127.0.0.1:8080`.
2. `fogd` validates + queues session runs via `internal/runner`.
3. `internal/runner` creates/uses worktrees via the `wtx` CLI.
4. `internal/runner` executes an AI tool adapter from `internal/ai`.
5. Progress is recorded to SQLite (`internal/state`) as run events (`run_events`).
6. UI consumes run history via polling and live updates via SSE.

## Session Semantics (Hard Invariants)

- One session = one branch + one worktree path (the "session worktree").
- A run is a state machine step within a session; runs append run events.
- Follow-up:
  - does not create a new worktree
  - stays on the same branch/worktree
  - reuses the tool conversation id when adapters provide it (stored as `ai_session` run event).
- Re-run:
  - schedules a new run with the same prompt (same session worktree).
- Fork:
  - creates a new session (new branch/worktree) based on the source session’s current worktree head
  - generates an AI summary from the source session and appends it to the fork prompt
  - starts a fresh tool conversation (no resume), but passes the summary context.
- Cancel:
  - only the latest active run can be canceled
  - cancellation stops the entire process group.

If a proposed change violates these invariants, treat it as a product change and clarify with the user.

## Storage Model

Fog home:
- `FOG_HOME` env var overrides the default `~/.fog` (see `internal/env/home.go`).
- SQLite DB: `FOG_HOME/fog.db` (repos, settings, secrets, sessions, runs, run_events, tasks).
- Master key: `FOG_HOME/master.key` (file-based AES-256-GCM key).

Managed repos layout (default):
- `FOG_HOME/repos/<owner>/<repo>/repo.git` (bare clone)
- `FOG_HOME/repos/<owner>/<repo>/base` (base worktree)

Encryption:
- Secrets are encrypted at rest in SQLite using AES-256-GCM with a local key file.
- Never store GitHub PATs or Slack tokens in plaintext.
- Avoid logging tokens; sanitize command args that may contain `Authorization` headers.

## AI Tool Adapters

Adapters live in `internal/ai/*` and must:
- detect availability without expensive operations
- prefer headless + streaming when supported
- fall back to plain output modes for compatibility
- stream output through `proc.RunStreaming` when possible
- return a stable `ConversationID` if the tool supports resume/follow-up

Supported tools (canonical names):
- `claude` (also accepts `claude-code`)
- `cursor`
- `gemini`
- `aider`

Tool detection notes:
- GUI-launched processes on macOS can have a limited PATH.
- Command resolution uses PATH plus common fallback dirs (Homebrew, `~/.local/bin`, etc.).

## API Surface (Local)

Primary endpoints used by desktop sessions:
- `GET /api/settings` / `PUT /api/settings`
- `GET /api/repos` / `POST /api/repos/discover` / `POST /api/repos/import`
- `GET /api/sessions` / `POST /api/sessions`
- `POST /api/sessions/{id}/runs` (follow-up)
- `POST /api/sessions/{id}/fork`
- `POST /api/sessions/{id}/cancel`
- `GET /api/sessions/{id}/diff`
- `POST /api/sessions/{id}/open`
- `GET /api/sessions/{id}/runs/{run_id}/events`
- `GET /api/sessions/{id}/runs/{run_id}/stream` (SSE)

## Desktop UI Notes

Frontend lives in `cmd/fogapp/frontend/*` (plain HTML/CSS/JS embedded into Wails).

- The UI uses polling for list/detail refresh.
- Active runs also open an SSE stream to append events live.
- The app ensures `fogd` is reachable; if not, it starts an embedded daemon.

## Where To Change Things (Common Cases)

- Session lifecycle + invariants: `internal/runner/session.go`
- Worktree creation (wtx invocation): `internal/runner/runner.go`
- Tool adapters + streaming: `internal/ai/*`
- Streaming process execution: `internal/proc/*`
- State schema + encryption: `internal/state/*`
- Local API routes: `internal/api/*`
- Desktop UI: `cmd/fogapp/frontend/*`, desktop shell: `cmd/fogapp/*`

## Do / Don’t

Do:
- keep changes small and reviewable (vertical slices)
- add tests for new behavior
- update docs in the same PR when behavior changes
- use `proc.Run` / `proc.RunStreaming` for cancellable long-running commands
- preserve user safety invariants (worktrees, no force-push)

Don’t:
- store secrets in plaintext or print them in logs/errors
- run AI in the user’s main working directory checkout
- introduce heavy dependencies for small problems
- silently change session semantics (follow-up/fork/cancel behavior)

## How To Make Changes (Slices)

Preferred workflow for non-trivial changes:

1. Define/confirm invariants and UX expectations.
2. Implement backend slice:
   - state changes (SQLite) only if needed
   - runner behavior + run events
   - API endpoint wiring
3. Add/adjust tests for the slice.
4. Implement UI slice (desktop) if user-facing.
5. Update docs (`README.md`, `docs/*`, `AGENTS.md`) to match.
6. Run targeted tests first, then `go test ./...`.

Commit strategy:
- One commit per slice when possible (Conventional Commits).
- Keep commits mechanically reviewable (avoid unrelated changes).

## Testing

Unit tests:
- `go test ./...`

Targeted packages:
- `go test ./internal/runner`
- `go test ./internal/api`
- `go test ./internal/ai`
- `go test ./cmd/fogapp`

Desktop frontend smoke test (headless; requires Chrome/Chromium):
- `go test ./cmd/fogapp -run TestDesktopFrontendSmokeFlows -count=1`

Use a dev home dir to avoid polluting real state:
- `FOG_HOME=/tmp/fog-dev go run ./cmd/fogd --port 8080`

## Debugging Tips

- Wrong behavior after code changes is often an old `fogd` process/binary.
  - Check port owner: `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - Confirm binary content: `strings bin/fogd | rg gemini`
- Tool not detected in desktop:
  - verify `GET /api/settings` includes `available_tools`
  - remember desktop UI caches settings until reload.

## Development Commands

- Build binaries: `make all`
- Install: `make install`
- Desktop dev: `make fogapp-dev` (requires `wails`)
- Desktop build: `make fogapp-build`

Build tags:
- Desktop uses `-tags desktop` (`cmd/fogapp` enforces this).

## Contributor Defaults

- Commit messages: Conventional Commits.
- Author identity is configured in git (`user.name` / `user.email`) and should remain consistent.

Fog home:
- `FOG_HOME` (default `~/.fog`)
- SQLite DB: `FOG_HOME/fog.db` (repos, settings, secrets, sessions, runs, run_events, tasks)
- Master key: `FOG_HOME/master.key` (file-based AES-256-GCM key)

Rules:
- Never store GitHub PATs in plaintext.
- Secrets are encrypted at rest using AES-GCM with a local key file (no keychain dependency).
- Avoid logging tokens; sanitize git args that may contain headers.

Managed repos layout (default):
- `FOG_HOME/repos/<owner>/<repo>/repo.git` (bare clone)
- `FOG_HOME/repos/<owner>/<repo>/base` (base worktree)

## Key Invariants

- Safety: operate in worktrees; do not mutate user repos in-place.
- Cancellation: only the latest active run is cancelable; cancellation stops the entire process group.
- PR creation: uses `gh` CLI; PR is created once per session (draft) when enabled; follow-ups update the same branch/PR.

## Where To Change Things

- AI tool adapters and streaming: `internal/ai/*`
- Session engine (follow-ups, fork, cancellation, run events): `internal/runner/session.go`
- HTTP API endpoints: `internal/api/*`
- State (SQLite schema, encryption): `internal/state/*`
- Desktop UI: `cmd/fogapp/frontend/*` (plain HTML/CSS/JS embedded into Wails)

## Development Commands

- Unit tests: `go test ./...`
- Build binaries: `make all`
- Desktop dev: `make fogapp-dev` (requires `wails`)
- Desktop build: `make fogapp-build`

Build tags:
- Desktop uses `-tags desktop` (`cmd/fogapp` enforces this).

## Notes For Other Agents

- Prefer small, reviewable changes and keep code simple.
- Avoid third-party deps unless there is a clear payoff.
- Keep docs in sync with behavior (especially session semantics and storage layout).
