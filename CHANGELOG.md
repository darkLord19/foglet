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

