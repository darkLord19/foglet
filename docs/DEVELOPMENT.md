# Development

## Requirements

- Go (use the version in `go.mod`)
- Git
- Wails CLI (for desktop dev/build): `wails`
- Optional (required for repo discovery/import): `gh` (GitHub CLI), an AI tool (`claude`, `cursor-agent`, `gemini`, `aider`)

## Build

Build all CLI binaries:

```bash
make all
```

Build a single binary:

```bash
go build ./cmd/fogd
go build ./cmd/fog
go build ./cmd/wtx
```

Desktop build uses the `desktop` build tag:

```bash
go build -tags desktop ./cmd/fogapp
```

## Test

Run all unit tests:

```bash
go test ./...
```

Desktop frontend smoke test (headless, requires Chrome/Chromium):

```bash
go test ./cmd/fogapp -run TestDesktopFrontendSmokeFlows -count=1
```

## Run Locally

Run the daemon:

```bash
go run ./cmd/fogd --port 8080
```

Desktop dev mode:

```bash
make fogapp-dev
```

## Repo Structure

- `internal/api`: HTTP API used by desktop + automation
- `internal/runner/session.go`: session lifecycle engine (follow-up, fork, cancel, run events)
- `internal/ai`: tool adapters + streaming helpers
- `internal/state`: SQLite schema + encrypted secrets
- `cmd/*`: binaries

## Debugging

Use a separate Fog home for development:

```bash
FOG_HOME=/tmp/fog-dev go run ./cmd/fogd --port 8080
```

If a desktop-launched daemon cannot find tools, it is usually a PATH issue (GUI apps can inherit a limited PATH). Tool detection includes fallbacks for common install locations.
