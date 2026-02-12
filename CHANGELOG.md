# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Interactive TUI with fuzzy search
- Core CLI commands (list, add, open, rm)
- Multi-editor support (VS Code, Cursor, Neovim, Claude Code, Vim)
- Worktree metadata storage in .git/wtx/
- Global configuration at ~/.config/wtx/
- JSON output for automation
- Process management for dev servers
- Port detection and tracking
- VS Code extension with tree view and quick switcher
- Claude Code MCP server
- Shell completions (bash, zsh)
- Installation script
- Comprehensive documentation

### Security
- Path traversal prevention in worktree operations
- Safe worktree deletion with uncommitted changes check

## [0.1.0] - 2024-02-12

### Initial Release
- First working version with core functionality
