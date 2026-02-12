# wtx MCP Server

Model Context Protocol (MCP) server for wtx Git worktree manager. Enables Claude Code to manage worktrees directly.

## Installation

### From npm (when published)
```bash
npm install -g wtx-mcp-server
```

### From source
```bash
cd plugins/claude-code
npm install
npm run build
npm link
```

## Usage

Add to your Claude Code configuration file (`~/.claude/config.json`):

```json
{
  "mcpServers": {
    "wtx": {
      "command": "wtx-mcp-server"
    }
  }
}
```

Or if using npx:

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

## Available Tools

### wtx_list_worktrees

List all worktrees in the repository.

**Parameters:** None

**Returns:** JSON array of worktrees

**Example:**
```
Claude: Let me check your worktrees
[uses wtx_list_worktrees]

You have 3 worktrees:
- main (clean)
- feature-auth (dirty, 2 commits ahead)
- bugfix-login (clean)
```

### wtx_switch_worktree

Switch to a different worktree.

**Parameters:**
- `name` (string): Name of the worktree

**Example:**
```
User: Switch to the auth feature branch
Claude: [uses wtx_switch_worktree with name="feature-auth"]
Switched to feature-auth
```

### wtx_create_worktree

Create a new worktree.

**Parameters:**
- `name` (string): Name for the new worktree
- `branch` (string, optional): Branch name (defaults to worktree name)

**Example:**
```
User: Create a worktree for the new API
Claude: [uses wtx_create_worktree with name="api-v2"]
Created worktree api-v2
```

### wtx_delete_worktree

Delete a worktree.

**Parameters:**
- `name` (string): Name of the worktree to delete

**Example:**
```
User: Delete the old feature branch
Claude: [uses wtx_delete_worktree with name="old-feature"]
Deleted worktree old-feature
```

## Requirements

- `wtx` CLI must be installed and in PATH
- Claude Code with MCP support

## Development

```bash
# Install dependencies
npm install

# Build
npm run build

# Watch mode
npm run watch
```

## How It Works

This MCP server is a thin wrapper around the `wtx` CLI. It:

1. Receives MCP tool calls from Claude Code
2. Executes corresponding `wtx` commands
3. Returns results in MCP format

This approach ensures:
- Simple implementation (just call the CLI)
- Always in sync with CLI behavior
- Easy to maintain

## Troubleshooting

### "wtx not found"

Make sure `wtx` is installed and in your PATH:

```bash
which wtx
# Should output: /path/to/wtx

# If not found, install:
go install github.com/darkLord19/foglet/cmd/wtx@latest
```

### Server not starting

Check Claude Code logs:
```bash
tail -f ~/.claude/logs/mcp-server.log
```

## License

MIT
