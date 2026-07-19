# Contributing

Fog is desktop-first and local-first. Keep changes small, secure, and easy to review.

## Development Setup

Requirements:
- Go (use the version in `go.mod`)
- Git

Desktop (optional, for `fogapp` dev/build):
- Wails CLI: `wails`

Helpful tools (optional):
- `gh` for PR creation flows
- an AI tool: `claude`/`claude-code`, `cursor-agent`, or `antigravity` (`agy`)

## Build

```bash
make all
```

Desktop dev:

```bash
make fogapp-dev
```

Desktop build:

```bash
make fogapp-build
```

## Test

```bash
go test ./...
```

Desktop frontend smoke (headless, requires Chrome/Chromium):

```bash
go test ./cmd/fogapp -run TestDesktopFrontendSmokeFlows -count=1
```

## Guidelines

- Prefer the standard library; avoid new deps unless there is a clear payoff.
- Security is non-negotiable:
  - never persist tokens in plaintext
  - avoid logging secrets (sanitize command args / headers)
- Keep UX and safety rules stable:
  - sessions are long-lived branch + worktree conversations
  - follow-ups and re-runs operate on the same session worktree
  - fork creates a new branch/worktree from the session head
- Add tests for new behavior.
- Keep docs in sync with behavior (`docs/`).

## Commit Messages

Use Conventional Commits:
- `feat: ...`
- `fix: ...`
- `docs: ...`
- `test: ...`
- `refactor: ...`

## License

By contributing, you agree your contributions will be licensed under AGPL-3.0-or-later.

