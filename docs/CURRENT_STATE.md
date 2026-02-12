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
  - repository identifier is fixed to `owner/repo-name`
- Config commands:
  - `fog config view`
  - `fog config set --default-tool ... --branch-prefix ...`
- UI launcher:
  - `fog ui` ensures `fogd` is running and opens browser.
- Session execution groundwork is implemented in the runner:
  - create session (one branch/worktree)
  - follow-up run execution on same worktree
  - run event logging in SQLite
  - auto-commit with AI-generated commit message (fallback to deterministic message)
  - push only when auto-PR is enabled (or session already has PR)
  - draft PR is created once per session, follow-ups update same branch

### `fogd` (control plane)
- HTTP API:
  - `GET /health`
  - `GET /api/tasks`
  - `GET /api/tasks/{id}`
  - `POST /api/tasks/create`
  - `GET /api/sessions`
  - `POST /api/sessions`
  - `GET /api/sessions/{id}`
  - `GET /api/sessions/{id}/runs`
  - `POST /api/sessions/{id}/runs`
  - `GET /api/sessions/{id}/runs/{run_id}/events`
  - `GET /api/repos`
  - `POST /api/repos/discover`
  - `POST /api/repos/import`
  - `GET /api/settings`
  - `PUT /api/settings`
- Built-in web UI served at `/`:
  - session activity dashboard
  - create session + follow-up actions
  - repo discovery/import and managed repo listing
  - default tool + branch prefix settings
- Slack integration:
  - HTTP slash-command mode (`/slack/command`)
  - Socket Mode (`@fog`) with thread follow-ups
- Session API defaults:
  - `POST /api/sessions` runs async by default and returns session/run ids
  - `POST /api/sessions/{id}/runs` runs async by default
  - branch name is generated from `branch_prefix` + prompt slug when `branch_name` is omitted

### `internal/cloud` (distribution foundation, not wired to a binary yet)
- Slack install/event server primitives are implemented:
  - `/health`
  - `/slack/install`
  - `/slack/oauth/callback`
  - `/slack/events`
- Multi-workspace install persistence:
  - one installation row per Slack workspace (`team_id`)
  - bot token encrypted at rest with local AES-GCM key file
- Pairing and routing metadata persistence primitives:
  - `(team_id, slack_user_id) -> device_id` pairings
  - per-thread session mapping (`team/channel/root_ts -> session_id`)
  - Slack event id dedupe storage
- Current app mention behavior:
  - unpaired user => ephemeral "not paired" response
  - paired user => placeholder ephemeral response (routing execution in next slices)

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
- `~/.fog/repos/<owner>/<repo-name>/repo.git` (bare)
- `~/.fog/repos/<owner>/<repo-name>/base` (base worktree)

State details:
- PAT stored encrypted at rest (never plaintext).
- Default tool and branch prefix stored in settings table.
- Managed repos stored in repos table.
- Legacy task runs stored in SQLite task table(s) and task-run payload records.
- Session-first schema is now available in SQLite:
  - `sessions`
  - `runs`
  - `run_events`
  This is the persistence base for the upcoming desktop session UI and follow-up workflow.
- Cloud foundation uses a separate SQLite file (`fogcloud.db`) and key file (`cloud.key`) in its configured data dir.

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
- End-to-end cloud relay (device claim/dispatch execution loop).
- PR comment rerun loop.
- Containerized task isolation.
- Production-ready team/multi-user auth model.

## Source of truth

If this file and older narrative docs differ, treat this file as source of truth for current behavior.
