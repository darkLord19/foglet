# Usage

Fog is a local-first orchestrator for AI coding tools. It runs your existing AI CLI(s) inside isolated Git worktrees and exposes control via a desktop app and a local HTTP API.

Fog does not ship an LLM.

## Installation

Build and install all Go binaries:

```bash
make install
```

Or install via Go:

```bash
go install github.com/darkLord19/foglet/cmd/{wtx,fog,fogd,fogcloud,fogapp}@latest
```

## Onboarding (CLI + Desktop)

Fog needs:
- GitHub CLI (`gh`) installed and authenticated to discover/import repos
- a default AI tool to run when you omit `tool`
- at least one installed supported AI CLI (`claude`, `cursor-agent`, `gemini`, or `aider`)

```bash
fog setup
```

What it does:
- checks that `gh` is installed and authenticated (`gh auth login`)
- detects installed tools and sets `default_tool`

You can also set values explicitly:

```bash
fog config set --default-tool gemini --branch-prefix fog
```

Desktop onboarding wizard:
- appears when `onboarding_required` is true
- step 1: verify GitHub CLI status (install + auth)
- step 2: choose default tool and model (tool-specific model list)
- step 3: optionally discover and import repositories

## Managed Repos

Fog manages repositories under `FOG_HOME` (default `~/.fog`) as bare clones with a base worktree.

Discover accessible repos:

```bash
fog repos discover
```

Import a subset:

```bash
fog repos import --select owner/repo,owner/another-repo
fog repos list
```

Import initializes:
- `~/.fog/repos/<owner>/<repo>/repo.git` (bare)
- `~/.fog/repos/<owner>/<repo>/base` (base worktree)

## Desktop Sessions (Recommended)

Start the desktop app in dev mode:

```bash
make fogapp-dev
```

The desktop app:
- ensures `fogd` is reachable at `http://127.0.0.1:8080`
- starts an embedded daemon if no external `fogd` is running

### Session Model

- Session = long-lived branch + worktree + tool/model settings.
- Run = one prompt execution inside a session.
- Follow-ups and re-runs operate on the same session worktree.
- Fork creates a new branch/worktree from the current session head.

### Follow-Up And Re-Run

Follow-ups:
- keep the same branch/worktree
- reuse tool conversation id when the adapter exposes one (`ai_session` run event)

Re-run:
- schedules a new run with the same prompt (same worktree)

### Fork

Fork is explicit and starts a new session:
- branch name is auto-generated from prompt when omitted
- a short context summary is generated from the source session and appended to the fork prompt
- tool conversation is fresh (no resume), but it receives the summary context

## Streaming Output

`fogd` persists chunk-level output as run events (`ai_stream`) and exposes a Server-Sent Events stream:

```bash
curl -N "http://127.0.0.1:8080/api/sessions/<session_id>/runs/<run_id>/stream"
```

The desktop app uses SSE for active runs and polling as a fallback.

## CLI One-Off Tasks (`fog run`)

`fog run` is a one-shot flow that creates a worktree for the task:

```bash
fog run \
  --repo owner/repo \
  --branch fog/jwt-auth \
  --tool gemini \
  --prompt "Add JWT auth" \
  --commit \
  --pr
```

## AI Tools

Fog executes tools you already installed:
- `claude` / `claude-code`
- `cursor-agent`
- `gemini`
- `aider`

Adapters prefer headless/streaming modes when available and fall back to plain output when needed.

## Local Storage

`FOG_HOME` defaults to `~/.fog`:
- `fog.db`: SQLite state (repos, settings, secrets, sessions/runs/events, tasks)
- `master.key`: local AES-256-GCM key used to encrypt secrets at rest

Secrets are never stored in plaintext.
