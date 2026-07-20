# Sandboxing

Fog runs third-party AI CLIs on the user's machine. Git worktrees isolate the
*repo*; they do not isolate the *machine*. This document covers what is
implemented today and the findings that shape what comes next.

## Implemented: host guard

`internal/sandbox` restricts every AI CLI invocation. See the "Host Guard"
section of `AGENTS.md` for the rules that must be preserved when changing it.

- **Filesystem**: a macOS seatbelt profile denies reads of `~/.ssh`, `~/.aws`,
  `~/.config/gh`, `~/.claude.json`, and Fog's `master.key`, `api.token`, and
  `fog.db`. Linux enforcement is not implemented (`guard_other.go` is a
  passthrough reporting `Applied == false`).
- **Environment**: agents receive an allowlist rather than the daemon's full
  environment, scoped per tool via `toolEnvPrefixes`.

Opt out with `FOG_DISABLE_HOST_GUARD=1`.

This is host-level hardening, not a sandbox. It stops credential *reading*; it
does not stop a compromised agent from using the network.

## Credential brokering spike

**Question:** can Fog broker LLM auth so the real credential never enters a
sandbox, or must the token be injected into the sandbox?

**Answer: brokering works.** Measured against `claude-cli/2.1.215` on macOS 26.5
by pointing `ANTHROPIC_BASE_URL` at a local listener.

Findings:

1. **Claude Code honours `ANTHROPIC_BASE_URL`.** Requests go to the configured
   endpoint as `POST /v1/messages?beta=true`.
2. **With subscription login it sends `Authorization: Bearer <token>`** — the
   OAuth credential out of the macOS Keychain (service `Claude Code-credentials`).
3. **With `ANTHROPIC_API_KEY` set it sends that value verbatim as `x-api-key`,
   without touching the Keychain.** This is the load-bearing result: a sandboxed
   agent can be handed a Fog-issued placeholder, and `fogd` swaps it for the real
   credential on the way out. The real token never enters the sandbox.

### Design this implies for Slice 3

- Sandbox gets `ANTHROPIC_BASE_URL` pointed at `fogd` and `ANTHROPIC_API_KEY`
  set to a per-session, budget-scoped token that Fog issues and can revoke.
- `fogd` validates that token, strips it, attaches the real
  `Authorization: Bearer` from the Keychain, and forwards upstream.
- Per-session cost caps and an audit trail come free, since every request
  crosses the broker.

### Implementation notes that will bite

- **`anthropic-beta` differs between auth modes.** In OAuth mode the header
  includes `oauth-2025-04-20`; in API-key mode it is absent. When the broker
  swaps `x-api-key` for an OAuth bearer token it must also re-add that beta flag,
  or the upstream request is unlikely to be accepted.
- **Setting `ANTHROPIC_API_KEY` disables claude.ai connectors.** The CLI warns
  that the key "takes precedence over your claude.ai login", so an org's
  connectors will not load inside the sandbox. Acceptable, but user-visible.
- **Not yet proven end to end.** The probe never forwarded upstream, so
  Anthropic's acceptance of a proxied OAuth bearer token is untested. Verify
  before building on it.

### Still open

- `cursor-agent` and `agy`: credential storage and whether either honours a
  base-URL override is unknown. Same probe method applies.
