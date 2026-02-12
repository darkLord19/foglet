# wtx VS Code Extension

Manage Git worktrees directly from VS Code.

## Features

- 🌳 **Tree View** - See all worktrees in the sidebar
- ⌨️ **Quick Switcher** - Switch worktrees with Cmd+Shift+W
- ➕ **Create Worktrees** - Create new worktrees from VS Code
- 🗑️ **Delete Worktrees** - Remove worktrees safely
- 🔄 **Auto-refresh** - Updates when window gains focus

## Requirements

- `wtx` CLI must be installed and in PATH
- Install: `go install github.com/yourusername/wtx/cmd/wtx@latest`

## Usage

### Quick Switcher

Press `Cmd+Shift+W` (Mac) or `Ctrl+Shift+W` (Windows/Linux) to open the quick switcher.

### Tree View

View all worktrees in the Explorer sidebar under "Worktrees".

- Click to open a worktree
- Right-click for delete option
- Click refresh icon to update

### Commands

All commands available in Command Palette (Cmd+Shift+P):

- `wtx: Switch Worktree` - Quick switcher
- `wtx: List Worktrees` - Show all in editor
- `wtx: Create Worktree` - Create new
- `wtx: Delete Worktree` - Remove existing
- `wtx: Refresh` - Refresh tree view

## Extension Settings

This extension uses wtx's global configuration at `~/.config/wtx/config.json`.

## Development

```bash
cd plugins/vscode
npm install
npm run compile

# Package
npm run package
```

## License

MIT
