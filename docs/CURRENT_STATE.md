# Fog Current State

This document reflects the currently implemented product surface in this repository.

## Components

### `wtx` (worktree CLI)
- Interactive TUI and command-based worktree management.
- `wtx add --json` for machine-readable automation.
- Metadata stored in git common dir (works correctly from worktrees).
- `wtx add` creates missing branch automatically.

### `fog` (orchestration CLI)
- Runs AI tasks in isolated worktrees.
- Task state persisted in SQLite (`~/.fog/fog.db`).
- Supports managed multi-repo execution with `--repo`.
- Onboarding via `fog setup`:
  - GitHub PAT validation and encrypted storage.
  - Default tool selection.
- Repo lifecycle:
  - `fog repos discover`
  - `fog repos import`
  - `fog repos list`
- Config commands:
  - `fog config view`
  - `fog config set --default-tool ... --branch-prefix ...`
- UI launcher:
  - `fog ui` ensures `fogd` is running and opens browser.

### `fogd` (control plane)
- HTTP API:
  - `GET /health`
  - `GET /api/tasks`
  - `GET /api/tasks/{id}`
  - `POST /api/tasks/create`
  - `GET /api/settings`
  - `PUT /api/settings`
- Built-in web UI served at `/`:
  - task activity dashboard
  - default tool + branch prefix settings
- Slack integration:
  - HTTP slash-command mode (`/slack/command`)
  - Socket Mode (`@fog`) with thread follow-ups

## Slack command contract

Initial task syntax:

```text
@fog [repo='' tool='' model='' autopr=true/false branch-name='' commit-msg=''] prompt
```

Rules:
- `repo` is required.
- All other keys are optional.
- Follow-ups in same thread are plain prompt only:

```text
@fog refine edge cases and add tests
```

## Data/storage model

Fog home:
- `~/.fog/fog.db` (SQLite)
- `~/.fog/master.key` (file-based local encryption key)
- `~/.fog/repos/<alias>/repo.git` (bare)
- `~/.fog/repos/<alias>/base` (base worktree)

State details:
- PAT stored encrypted at rest (never plaintext).
- Default tool and branch prefix stored in settings table.
- Managed repos stored in repos table.
- Task runs stored in SQLite task table(s).

## Distribution

- Release artifact builder script:
  - `scripts/release/build-artifacts.sh`
- Homebrew formula generator:
  - `scripts/release/generate-homebrew-formula.sh`
- Release workflow:
  - `.github/workflows/release.yml` (tag `v*`)
- Linux installer:
  - `scripts/install-linux.sh` (checksum verify + version pin)

## AI tools

Supported adapters:
- `claude`
- `aider`
- `cursor` (headless via `cursor-agent`)

Default tool must be explicitly configured, otherwise API/Slack/CLI task creation fails when tool is omitted.

## What is intentionally not implemented yet

- Full OAuth onboarding (PAT-only today).
- PR comment rerun loop.
- Containerized task isolation.
- Production-ready team/multi-user auth model.

## Source of truth

If this file and older narrative docs differ, treat this file as source of truth for current behavior.
