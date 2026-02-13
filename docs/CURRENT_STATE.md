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
  - `fog app` launches desktop app binary (`fogapp`) when installed.
- Session execution groundwork is implemented in the runner:
  - create session (one branch, one run worktree)
  - follow-up and re-run execution in new worktrees on the same branch
  - run event logging in SQLite
  - real process cancellation for active runs
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
  - `POST /api/sessions/{id}/cancel`
  - `GET /api/sessions/{id}/diff`
  - `POST /api/sessions/{id}/open`
  - `GET /api/repos`
  - `POST /api/repos/discover`
  - `POST /api/repos/import`
  - `GET /api/settings`
  - `PUT /api/settings`
  - `GET /api/cloud`
  - `PUT /api/cloud`
  - `POST /api/cloud/pair`
  - `POST /api/cloud/unpair`
- Slack integration:
  - HTTP slash-command mode (`/slack/command`)
  - Socket Mode (`@fog`) with thread follow-ups
- Optional cloud relay mode:
  - configure cloud URL + pairing in local API/UI
  - `fogd` claims cloud jobs for the paired device and executes them through session runner
  - completion is posted back through cloud API and thread mapping is preserved

### `fogapp` (Wails desktop preview)
- Wails desktop shell scaffold exists at `cmd/fogapp`.
- Startup behavior:
  - ensures local `fogd` API server is running on `127.0.0.1:8080`
  - starts bundled in-process `fogd` server when an external one is not already running
  - loads a desktop-focused UI over existing Fog HTTP APIs
- Desktop UI supports:
  - running/completed session sidebar
  - first-line prompt session titles
  - new session composer (`repo`, `tool`, `model`, `branch_name`, `autopr`, prompt)
  - repo discover/import
  - settings update (`default_tool`, `branch_prefix`)
  - detail view tabs (`timeline`, `diff`, `logs`, `stats`)
  - run actions: `Stop`, `Re-run`, `Open in Editor`
- Current status:
  - Desktop build uses `desktop` build tag and Wails CLI workflow.
  - Linux AppImage packaging is wired into release artifacts via optional build flag.
  - desktop frontend smoke tests cover API-mocked UI flows.
- Session API defaults:
  - `POST /api/sessions` runs async by default and returns session/run ids
  - `POST /api/sessions/{id}/runs` runs async by default
  - branch name is generated from `branch_prefix` + prompt slug when `branch_name` is omitted

### `fogcloud` (distribution control plane)
- Dedicated binary at `cmd/fogcloud`.
- Runs cloud Slack install/event routing over `internal/cloud`.
- Exposes:
  - `/health`
  - `/slack/install`
  - `/slack/oauth/callback`
  - `/slack/events`
  - `POST /v1/pair/claim`
  - `POST /v1/pair/unpair`
  - `POST /v1/device/jobs/claim`
  - `POST /v1/device/jobs/{id}/complete`

### `internal/cloud` (distribution foundation)
- Slack install/event server primitives are implemented:
  - `/health`
  - `/slack/install`
  - `/slack/oauth/callback`
  - `/slack/events`
- Device routing APIs are implemented:
  - `POST /v1/pair/claim`
  - `POST /v1/pair/unpair`
  - `POST /v1/device/jobs/claim`
  - `POST /v1/device/jobs/{id}/complete`
- Multi-workspace install persistence:
  - one installation row per Slack workspace (`team_id`)
  - bot token encrypted at rest with local AES-GCM key file
- Pairing and routing metadata persistence primitives:
  - `(team_id, slack_user_id) -> device_id` pairings
  - one-time pairing requests (`code`)
  - per-device auth tokens (hashed at rest)
  - queued jobs per device
  - per-thread session mapping (`team/channel/root_ts -> session_id`)
  - Slack event id dedupe storage
- Current app mention behavior:
  - unpaired user => ephemeral pairing code response
  - paired user => parse/validate command and enqueue a job
  - follow-up thread mention => requires existing thread session mapping
  - device completion endpoint writes thread session mapping for initial runs and posts completion in Slack

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
  This is used by session APIs, follow-up workflow, cancellation, and desktop timeline/diff/log UI.
- Cloud foundation uses a separate SQLite file (`fogcloud.db`) and key file (`cloud.key`) in its configured data dir.
- Local fogd cloud pairing state is persisted in `~/.fog/fog.db`:
  - settings: `cloud_url`, `cloud_device_id`
  - encrypted secret: `cloud_device_token`

## Distribution

- Release artifact builder script:
  - `scripts/release/build-artifacts.sh`
  - optional `fogapp` AppImage build via `BUILD_FOGAPP_APPIMAGE=true`
  - AppImage hashes are appended to the release checksums file
- Homebrew formula generator:
  - `scripts/release/generate-homebrew-formula.sh`
- Release workflow:
  - `.github/workflows/release.yml` (tag `v*`) publishes tarballs + AppImage assets
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
