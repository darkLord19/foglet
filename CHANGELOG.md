# Changelog

This project is early and changes quickly. Releases are tagged `v*`.

## Unreleased

- Desktop-first session UX backed by local `fogd` API.
- Follow-ups and re-runs operate in the session worktree.
- Explicit fork flow creates a new branch/worktree from the session head.
- Chunk-level streaming output persisted as run events + SSE streaming endpoint.
- Antigravity CLI (`agy`) adapter alongside Claude Code and Cursor Agent (replaces the deprecated Gemini CLI adapter).
- Encrypted PAT storage in local SQLite (`~/.fog/fog.db` + `~/.fog/master.key`).

