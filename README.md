# wtx - Git Worktree Manager

> "Cmd+Tab for your Git branches"

Fast, keyboard-driven workspace switcher for Git worktrees.

## Features

- 🚀 **Instant switching** - Switch between worktrees in <2 seconds
- 🎨 **Beautiful TUI** - Interactive fuzzy finder with status indicators  
- 🔧 **Multi-editor** - VS Code, Cursor, Neovim, Claude Code, Vim support
- 🌐 **Dev server tracking** - Manage ports and processes per worktree
- 🔒 **Safe by default** - Never lose uncommitted work
- ⌨️ **Keyboard-first** - Everything accessible without mouse
- 🤖 **Claude-friendly** - JSON output for automation and AI assistants

## Installation

### Go Install
```bash
go install github.com/yourusername/wtx/cmd/wtx@latest
```

### From Source
```bash
git clone https://github.com/yourusername/wtx
cd wtx
make install
```

### Homebrew (coming soon)
```bash
brew install yourusername/tap/wtx
```

## Quick Start

```bash
# Interactive switcher (default)
wtx

# List all worktrees
wtx list

# Create new worktree
wtx add feature-auth

# Open worktree in editor
wtx open feature-auth

# Remove worktree
wtx rm old-feature
```

## Interactive Mode

Just run `wtx` to launch the TUI:

```
┌─ Worktree Manager ─────────────────────┐
│ > main                                  │
│   auth-refactor  ● dirty • ↑2          │
│   bugfix-login   ✓ clean               │
│   api-v2         ✓ clean               │
└─────────────────────────────────────────┘

Press Enter to open | q to quit | / to search
```

### Keyboard Shortcuts

- `Enter` - Open selected worktree in editor
- `/` or `Ctrl+F` - Start fuzzy search  
- `↑/↓` or `j/k` - Navigate
- `r` - Refresh list
- `q` or `Ctrl+C` - Quit

## CLI Commands

### `wtx list`
List all worktrees with status

```bash
# Human-readable
wtx list

# JSON output (for scripts/AI)
wtx list --json
```

### `wtx add <name> [branch]`
Create a new worktree

```bash
# Create from existing branch
wtx add feature-auth

# Create new branch
wtx add new-feature main

# Worktree created at ../worktrees/feature-auth by default
```

### `wtx open <name>`
Open worktree in your editor

```bash
# Use configured editor
wtx open feature-auth

# Override editor
wtx open feature-auth --editor vscode
```

### `wtx rm <name>`
Remove a worktree safely

```bash
# Interactive safety check for dirty worktrees
wtx rm old-feature

# Force remove
wtx rm old-feature --force
```

## Configuration

Config stored in `~/.config/wtx/config.json`

```json
{
  "editor": "cursor",
  "reuse_window": true,
  "worktree_dir": "../worktrees",
  "auto_start_dev": false,
  "default_branch": "main"
}
```

### Editor Support

Supported editors (auto-detected):
- VS Code (`code`)
- Cursor (`cursor`)  
- Neovim (`nvim`)
- Claude Code (`claude`)
- Vim (`vim`)

Set preferred editor:
```bash
# Via config
editor="vscode"

# Via environment variable
export EDITOR=nvim

# Via flag
wtx open main --editor cursor
```

## Metadata Storage

Worktree metadata stored in `.git/wtx/metadata.json` (not cleaned by git)

Tracks:
- Creation timestamp
- Last opened timestamp  
- Dev command
- Ports in use
- Notes

## Use with Claude (AI)

wtx is designed to work seamlessly with Claude via computer use:

```
User: "Switch to my auth feature branch"
Claude: [bash: wtx open auth-refactor]

User: "What worktrees do I have?"
Claude: [bash: wtx list --json]
```

### JSON Output Format

```bash
$ wtx list --json
[
  {
    "name": "main",
    "path": "/Users/dev/project",
    "branch": "main",
    "head": "abc123",
    "locked": false,
    "prunable": false
  }
]
```

## Editor Extensions

### VS Code / Cursor

Install the wtx extension from the marketplace:

```bash
code --install-extension wtx
```

Features:
- Tree view of all worktrees
- Quick switcher (Cmd+Shift+W)
- Create/delete from sidebar
- Status indicators

### Claude Code (MCP Server)

Add to your Claude Code config:

```json
{
  "mcpServers": {
    "wtx": {
      "command": "npx",
      "args": ["-y", "wtx-mcp-server"]
    }
  }
}
```

Available tools:
- `wtx_list_worktrees` - List all worktrees
- `wtx_switch_worktree` - Switch to a worktree
- `wtx_create_worktree` - Create new worktree

## Development

```bash
# Build
make build

# Run tests
make test

# Install locally
make install

# Run locally
make dev
```

### Project Structure

```
wtx/
├── cmd/wtx/          # CLI entry point
├── internal/
│   ├── git/          # Git operations
│   ├── tui/          # Interactive UI
│   ├── editor/       # Editor adapters
│   ├── metadata/     # Data storage
│   ├── config/       # Configuration
│   ├── process/      # Process management
│   └── util/         # Utilities
└── plugins/
    ├── vscode/       # VS Code extension
    └── claude-code/  # Claude Code MCP server
```

## Roadmap

- [x] Core CLI (list, add, remove, open)
- [x] Interactive TUI
- [x] Multi-editor support
- [x] JSON output for automation
- [ ] Dev server management
- [ ] Port tracking
- [ ] Templates
- [ ] `wtx doctor` health checks
- [ ] Shell completions
- [ ] VS Code extension
- [ ] Claude Code MCP server

## Contributing

Contributions welcome! Please open an issue or PR.

## License

MIT

---

**Made with ❤️ for developers who love worktrees**
