# Changelog

This project is early and changes quickly. Releases are tagged `v*`.

## Unreleased

- Desktop-first session UX backed by local `fogd` API.
- Follow-ups and re-runs operate in the session worktree.
- Explicit fork flow creates a new branch/worktree from the session head.
- Chunk-level streaming output persisted as run events + SSE streaming endpoint.
- Antigravity CLI (`agy`) adapter alongside Claude Code and Cursor Agent (replaces the deprecated Gemini CLI adapter).
- Encrypted PAT storage in local SQLite (`~/.fog/fog.db` + `~/.fog/master.key`).
- Host guard for agent processes: AI CLIs now run under a macOS seatbelt profile
  denying reads of `~/.ssh`, `~/.aws`, `~/.config/gh`, `~/.claude.json`, and
  Fog's own `master.key`, `api.token`, and `fog.db`. Previously any agent run
  could read the key that decrypts stored GitHub and Slack tokens. Set
  `FOG_DISABLE_HOST_GUARD=1` to opt out. Linux enforcement is not implemented
  yet.
- Agent processes now receive a filtered environment instead of inheriting the
  daemon's. Shell/locale basics, proxy and CA settings, Node runtime vars, and
  the running tool's own credential prefixes are kept; everything else is
  dropped, so `AWS_*`, `GITHUB_TOKEN`, `DATABASE_URL`, `NODE_AUTH_TOKEN` and
  `GOOGLE_APPLICATION_CREDENTIALS` no longer reach agents. Credentials are
  scoped per tool, so a Claude run cannot read `CURSOR_API_KEY`. Covered by the
  same `FOG_DISABLE_HOST_GUARD=1` opt-out.

