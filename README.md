# Fog

Turn your local machine into cloud agents.

Fog is a local-first orchestration layer for running existing AI coding CLIs against your repositories in isolated Git worktrees.

Fog does not ship an LLM.

## What You Get

- `wtx`: Git worktree manager (no AI, no networking).
- `fog`: CLI for onboarding, managed repos, and one-off tasks.
- `fogd`: local daemon (HTTP API + session execution engine).
- `fogapp`: Wails desktop UI that talks to `fogd` and starts it when needed.

## Sessions (Desktop)

A session is a long-lived branch/worktree "conversation" that you can follow up on.

- Follow-ups and re-runs operate on the same session worktree.
- Forking is explicit: it creates a new branch/worktree from the current session head.
- Run output is persisted as events; active runs can be streamed via SSE.

## Supported AI Tools

Fog executes tools you already installed:
- `claude` / `claude-code`
- `cursor` (requires `cursor-agent` binary)
- `gemini`
- `aider`

Adapters prefer headless/streaming modes when available and fall back to plain output when needed.

## Quick Start

Install:

```bash
make install
# or:
go install github.com/darkLord19/foglet/cmd/{wtx,fog,fogd,fogcloud,fogapp}@latest
```

Onboard via CLI (GitHub PAT + default tool):

```bash
fog setup
```

Import one repo:

```bash
fog repos discover
fog repos import --select owner/repo
```

Run the desktop app (dev mode):

```bash
make fogapp-dev
```

On first desktop launch, Fog shows an onboarding wizard that walks through:
- GitHub PAT
- default tool + tool-specific default model
- optional repo import

Desktop onboarding requires at least one installed supported AI CLI.

## Local Storage And Security

Fog home:
- `FOG_HOME` (default `~/.fog`)
- `FOG_HOME/fog.db` (SQLite state: repos, settings, sessions, runs, run events, tasks)
- `FOG_HOME/master.key` (AES-256-GCM key for encrypting secrets at rest)
- `FOG_HOME/repos/...` (bare clones + base worktrees)

Fog never stores your GitHub PAT in plaintext.

## Docs

- `docs/README.md`
- `docs/USAGE.md`
- `docs/API.md`
- `docs/DEVELOPMENT.md`
- `docs/RELEASE.md`

## License

AGPL-3.0-or-later (see `LICENSE`).
