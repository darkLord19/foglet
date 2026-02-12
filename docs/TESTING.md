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
GOCACHE=/tmp/go-build go test ./internal/state
GOCACHE=/tmp/go-build go test ./internal/runner
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
  - `~/.fog/repos/<alias>/repo.git`
  - `~/.fog/repos/<alias>/base`

## 4. fog run task test

```bash
fog run \
  --repo <alias> \
  --branch fog/test-flow \
  --prompt "Create a tiny README change for smoke test" \
  --commit
```

Optional validation/PR:

```bash
fog run \
  --repo <alias> \
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
    "repo":"<alias>",
    "branch":"fog/api-smoke",
    "prompt":"Apply a tiny formatting fix",
    "ai_tool":"claude",
    "options":{"commit":false,"async":true}
  }'
```

## 6. Web UI test

```bash
fog ui
```

Expected checks:
- opens `http://127.0.0.1:8080/`
- shows active/total tasks
- lists repo + branch + state + tool
- settings form updates `default_tool` and `branch_prefix`

## 7. Slack HTTP mode test

Start daemon:

```bash
fogd --port 8080 --enable-slack --slack-mode http --slack-secret <secret>
```

Local payload smoke test:

```bash
curl -X POST http://localhost:8080/slack/command \
  -d "text=[repo='<alias>' tool='claude'] add smoke test note" \
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
  - `@fog [repo='<alias>' tool='claude'] implement small feature`
- Follow-up in thread:
  - `@fog add tests for edge case`

Expected checks:
- bot acks and posts progress/completion in thread.
- follow-up reuses thread context and launches another task.

## 9. Release packaging smoke test

From repo root:

```bash
scripts/release/build-artifacts.sh v0.0.0-test dist
```

Expected checks:
- archives generated for linux/darwin amd64/arm64
- checksum file generated
- Homebrew formula generation succeeds:

```bash
scripts/release/generate-homebrew-formula.sh v0.0.0-test dist/wtx_0.0.0-test_checksums.txt
```

## 10. Linux installer smoke test

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
