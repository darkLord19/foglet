# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added - Fog System
- **fog CLI** - AI orchestration with task lifecycle management
- **fogd daemon** - HTTP API server with Slack integration
- **AI tool adapters** - Support for Claude Code, Aider, Cursor
- **Task tracking** - Persistent task and settings state in `~/.fog/fog.db`
- **Async execution** - Fire-and-forget task running
- **HTTP API** - RESTful endpoints for task management
- **Slack integration** - Slash command `/fog` for team collaboration
- **Auto-commit** - Automatic git commit after AI completion
- **Auto-PR** - Create pull requests via GitHub CLI
- **Validation** - Run tests after AI task completes
- **Setup hooks** - Auto-run commands after worktree creation
- **SQLite state store** - Central state in `~/.fog/fog.db` (modernc.org/sqlite)
- **Encrypted PAT persistence** - File-key AES-GCM encryption for GitHub token storage

### Added - wtx Enhancements
- `wtx status <n>` - Detailed worktree status
- `wtx config` - View configuration
- Setup command execution after creating worktree
- Validation command support
- Enhanced metadata tracking (setup status, validation)
- Command execution utility

### Added - Documentation
- Complete Fog guide (docs/FOG.md)
- API documentation
- Slack integration guide
- Task lifecycle documentation
- Architecture diagrams

### Security
- Path traversal prevention in worktree operations
- Safe worktree deletion with uncommitted changes check
- GitHub PAT (if used) is stored encrypted at rest, never plaintext
- Local-only task execution

## [0.1.0] - 2024-02-12

### Initial Release
- **wtx** - Core worktree management
- Interactive TUI with fuzzy search
- Multi-editor support (VS Code, Cursor, Neovim, Claude Code, Vim)
- Metadata storage in .git/wtx/
- Global configuration
- JSON output for automation
- Process management for dev servers
- VS Code extension
- Claude Code MCP server
- Shell completions (bash, zsh)
- Comprehensive documentation
