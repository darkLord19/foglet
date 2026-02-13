# Testing Guide

This guide covers automated and manual testing for all major components.

## Prerequisites

- Go 1.24+
- Git
- Optional (for full feature tests):
  - `gh` (GitHub CLI) for PR creation flows
  - `cursor-agent`, `claude`, or `aider`
  - Slack app credentials for live Slack tests

## 1. Automated tests

Run all tests:

```bash
GOCACHE=/tmp/go-build go test ./...
```

Run focused suites:

```bash
GOCACHE=/tmp/go-build go test ./cmd/fog
GOCACHE=/tmp/go-build go test ./cmd/fogd
GOCACHE=/tmp/go-build go test ./internal/api
GOCACHE=/tmp/go-build go test ./internal/slack
GOCACHE=/tmp/go-build go test ./internal/cloud
GOCACHE=/tmp/go-build go test ./internal/state
GOCACHE=/tmp/go-build go test ./internal/runner
```

Session persistence focus:

```bash
GOCACHE=/tmp/go-build go test ./internal/state -run Session
GOCACHE=/tmp/go-build go test ./internal/runner -run Session
```

## 2. wtx manual test

Inside any git repo:

```bash
wtx list
wtx add test-worktree
wtx add --json test-worktree-json
wtx status test-worktree
wtx open test-worktree
```

Expected checks:
- `wtx add` creates a worktree path.
- `wtx add --json` returns JSON including path.
- metadata continues to work from both repo root and worktree path.

## 3. fog setup + repo registry test

```bash
fog setup
fog repos discover
fog repos import
fog repos list
fog config view
```

Expected checks:
- PAT is accepted and persisted encrypted.
- default tool is set.
- selected repos appear in `fog repos list`.
- each imported repo has:
  - `~/.fog/repos/<owner>/<repo-name>/repo.git`
  - `~/.fog/repos/<owner>/<repo-name>/base`

## 4. fog run task test

```bash
fog run \
  --repo <owner/repo-name> \
  --branch fog/test-flow \
  --prompt "Create a tiny README change for smoke test" \
  --commit
```

Optional validation/PR:

```bash
fog run \
  --repo <owner/repo-name> \
  --branch fog/test-validate \
  --prompt "Refactor small helper" \
  --validate \
  --validate-cmd "go test ./..." \
  --commit \
  --pr
```

Expected checks:
- new worktree is created via `wtx`.
- task state transitions are visible in `fog list` and `fog status`.
- commit is created when `--commit` is set.

## 5. fogd API test

Start daemon:

```bash
fogd --port 8080
```

In another terminal:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/tasks
curl http://localhost:8080/api/settings
curl -X PUT http://localhost:8080/api/settings \
  -H "Content-Type: application/json" \
  -d '{"default_tool":"claude","branch_prefix":"fog"}'
```

Create task API test:

```bash
curl -X POST http://localhost:8080/api/tasks/create \
  -H "Content-Type: application/json" \
  -d '{
    "repo":"<owner/repo-name>",
    "branch":"fog/api-smoke",
    "prompt":"Apply a tiny formatting fix",
    "ai_tool":"claude",
    "options":{"commit":false,"async":true}
  }'
```

Create session API test (async default):

```bash
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "repo":"<owner/repo-name>",
    "prompt":"Implement a small logging improvement",
    "tool":"claude",
    "autopr":false
  }'
```

Inspect sessions + runs:

```bash
curl http://localhost:8080/api/sessions
curl http://localhost:8080/api/sessions/<session_id>
curl http://localhost:8080/api/sessions/<session_id>/runs
curl -X POST http://localhost:8080/api/sessions/<session_id>/runs \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Follow up: add tests"}'
curl http://localhost:8080/api/sessions/<session_id>/runs/<run_id>/events
curl -X POST http://localhost:8080/api/sessions/<session_id>/cancel
curl http://localhost:8080/api/sessions/<session_id>/diff
curl -X POST http://localhost:8080/api/sessions/<session_id>/open
```

Expected checks:
- initial session create returns `session_id` and `run_id`
- follow-up call returns a new `run_id` for the same `session_id`
- follow-up/re-run allocates a new run worktree path
- run events include setup/ai/commit phases and terminal state
- cancel endpoint marks latest run as `CANCELLED`
- diff endpoint returns base-vs-branch diff for latest run worktree

## 6. Wails Desktop UI smoke test

Prerequisites:
- Wails CLI installed
- local desktop environment available (not headless CI)

Run:

```bash
make fogapp-dev
```

Expected checks:
- app starts and `fogd` is auto-started when not already running
- desktop dashboard renders session/repo/settings panels
- if PAT/default tool are missing, onboarding fields are shown first
- onboarding accepts GitHub PAT + default tool and persists both
- new session + follow-up actions work against local fogd APIs
- stop action cancels the latest active run
- re-run creates a new run/worktree in the same session branch
- detail tabs render timeline/diff/logs/stats

## 6.1 Desktop frontend E2E smoke tests (headless)

Prerequisites:
- Chrome/Chromium installed in PATH (`google-chrome`, `google-chrome-stable`, `chromium`, or `chromium-browser`)

Run:

```bash
GOCACHE=/tmp/go-build go test ./cmd/fogapp -run TestDesktopFrontendSmokeFlows -count=1
```

Expected checks:
- mocked API server receives create-session + follow-up + cancel + open + discover/import + settings calls
- frontend detail view loads initial run/events and tab content
- test skips automatically if no Chrome/Chromium binary is available

## 7. Slack HTTP mode test

Start daemon:

```bash
fogd --port 8080 --enable-slack --slack-mode http --slack-secret <secret>
```

Local payload smoke test:

```bash
curl -X POST http://localhost:8080/slack/command \
  -d "text=[repo='<owner/repo-name>' tool='claude'] add smoke test note" \
  -d "channel_id=C123" \
  -d "response_url=https://example.com/response"
```

Expected checks:
- immediate ack payload returned.
- task executes asynchronously.

## 8. Slack Socket Mode test

Start daemon:

```bash
fogd --port 8080 \
  --enable-slack \
  --slack-mode socket \
  --slack-bot-token xoxb-... \
  --slack-app-token xapp-...
```

In Slack:
- Initial:
  - `@fog [repo='<owner/repo-name>' tool='claude'] implement small feature`
- Follow-up in thread:
  - `@fog add tests for edge case`

Expected checks:
- bot acks and posts progress/completion in thread.
- follow-up reuses thread context and launches another task.

## 9. Cloud foundation tests

Automated:

```bash
GOCACHE=/tmp/go-build go test ./internal/cloud
```

Manual (install/event routes):

```bash
fogcloud \
  --port 9090 \
  --public-url https://fog-cloud.example \
  --slack-client-id <id> \
  --slack-client-secret <secret> \
  --slack-signing-secret <secret>

curl http://localhost:9090/health
curl -i http://localhost:9090/slack/install
```

Expected checks:
- `/health` returns JSON status payload.
- `/slack/install` redirects to Slack OAuth with client_id/state/redirect_uri.
- OAuth callback persists encrypted workspace bot token in cloud store.
- duplicate Slack event ids are ignored.
- unpaired `app_mention` generates a one-time pairing code response.
- paired `app_mention` queues a job claimable via `POST /v1/device/jobs/claim`.
- `POST /v1/device/jobs/{id}/complete` posts completion into the Slack thread and stores thread→session mapping.

## 10. Release packaging smoke test

From repo root:

```bash
scripts/release/build-artifacts.sh v0.0.0-test dist

# optional: include fogapp AppImage artifact
BUILD_FOGAPP_APPIMAGE=true scripts/release/build-artifacts.sh v0.0.0-test dist
```

Expected checks:
- archives generated for linux/darwin amd64/arm64
- checksum file generated
- with `BUILD_FOGAPP_APPIMAGE=true`, `dist/fogapp_0.0.0-test_linux_amd64.AppImage` is generated
- AppImage hash is included in `dist/wtx_0.0.0-test_checksums.txt`
- Homebrew formula generation succeeds:

```bash
scripts/release/generate-homebrew-formula.sh v0.0.0-test dist/wtx_0.0.0-test_checksums.txt
```

## 11. Linux installer smoke test

Run this section on a Linux host.

Dry-run latest release resolution:

```bash
scripts/install-linux.sh --dry-run
```

Version-pinned dry-run:

```bash
scripts/install-linux.sh --version v0.1.0 --dry-run
```

Expected checks:
- script resolves OS/arch correctly
- script prints selected version and URLs
- no secrets are logged
