# Fog - Local AI Agent Orchestration

> "Turn your local machine into cloud agents"

**Domain:** getfog.dev

Fog orchestrates AI coding tasks using existing AI tools in isolated Git worktrees. Safe, local, and async.

## 🎯 What is Fog?

Fog is a **local-first developer system** that:
- Runs AI coding tasks in **isolated worktrees**
- Supports **Cursor, Claude Code, Aider**
- Provides **CLI, HTTP API, and Slack** interfaces
- Creates **clean PRs** automatically
- Executes tasks **asynchronously**

## 🏗️ System Components

### 1. wtx - Worktree CLI
Git worktree manager (zero AI, zero networking)

```bash
wtx                    # Interactive TUI
wtx list              # List worktrees
wtx add <n>          # Create worktree  
wtx add --json <n>   # Create worktree (machine-readable output)
wtx open <n>         # Open in editor
wtx status <n>       # Detailed status
wtx config           # View configuration
```

### 2. fog - AI Orchestration CLI
Execute AI tasks locally

```bash
fog run \
  --branch feature-auth \
  --tool claude \
  --prompt "Add JWT authentication" \
  --commit \
  --pr

fog list              # List tasks
fog status <id>       # Task status
```

### 3. fogd - Control Plane
Daemon with HTTP API + Slack

```bash
fogd --port 8080 --enable-slack
```

## 🚀 Quick Start

### Installation

```bash
# Install all components
make install

# Or via Go
go install github.com/darkLord19/wtx/cmd/{wtx,fog,fogd}@latest
```

### Basic Usage

```bash
# 0. One-time onboarding (PAT + default tool)
fog setup

# 1. Simple AI task
fog run \
  --repo acme-api \
  --branch feature-otp \
  --tool claude \
  --prompt "Add OTP login" \
  --commit
# --tool is optional once default_tool is configured by `fog setup`

# 2. With validation
fog run \
  --repo acme-api \
  --branch fix-bug \
  --tool aider \
  --prompt "Fix auth bug" \
  --setup-cmd "npm ci" \
  --validate-cmd "npm test" \
  --commit \
  --pr

# 3. Start daemon
fogd --port 8080

# 3b. Open UI (starts fogd automatically if needed)
fog ui

# 4. Discover/import repos via configured PAT
fog repos discover
fog repos import
# import registers repo metadata and initializes:
# ~/.fog/repos/<alias>/repo.git (bare clone)
# ~/.fog/repos/<alias>/base (base worktree)
```

### Slack Usage

```
/fog [repo='acme-api' tool='claude' autopr=true branch-name='feature-search' commit-msg='add search'] implement full-text search
```

→ Response:
```
✅ Task completed: feature-search
Duration: 2m 30s
[Open Branch] [Create PR]
```

## ✨ Features

### wtx (Worktree Management)
- 🎨 **Interactive TUI** - Fuzzy search and keyboard navigation
- 🔧 **Multi-editor** - VS Code, Cursor, Neovim, Claude Code
- ⚙️ **Setup hooks** - Auto-run `npm install` after creation
- 📊 **Status tracking** - Dirty, ahead/behind, stash detection
- 🔒 **Safe operations** - Never lose uncommitted work

### fog (AI Orchestration)
- 🤖 **Multi-AI** - Cursor, Claude Code, Aider support
- 🌳 **Isolation** - Each task in separate worktree
- ✅ **Validation** - Run tests after AI
- 📝 **Auto-commit** - Commit changes automatically
- 🔀 **Auto-PR** - Create pull requests via `gh`
- 📊 **Lifecycle tracking** - Full state machine

### fogd (Control Plane)
- 🌐 **HTTP API** - RESTful task management
- 💬 **Slack** - Option-based command parsing for async task launch
- 🖥️ **Web UI launcher** - `fog ui` auto-starts fogd if not running
- 🔄 **Async** - Fire-and-forget execution
- 📢 **Notifications** - Completion alerts
- 🔌 **Extensible** - Easy to add integrations

## 📚 Documentation

- **[Complete Fog Guide](docs/FOG.md)** - Full documentation
- **[Project Summary](PROJECT_SUMMARY.md)** - Implementation details
- **[Contributing](CONTRIBUTING.md)** - Development guide
- **[Changelog](CHANGELOG.md)** - Version history

## 🛠️ Configuration

### wtx (~/.config/wtx/config.json)

```json
{
  "editor": "cursor",
  "reuse_window": true,
  "worktree_dir": "../worktrees",
  "setup_cmd": "npm install",
  "validate_cmd": "npm test",
  "default_branch": "main"
}
```

### fog state (~/.fog)

- State DB: `~/.fog/fog.db`
- Local encryption key: `~/.fog/master.key`
- GitHub PAT (if configured): encrypted at rest in SQLite
- Managed repository registry: stored in SQLite and used by fogd for multi-repo tasks

## 🎯 Use Cases

### Solo Developer
```bash
# Work on multiple features in parallel
fog run --branch feature-a --tool claude --prompt "..."
fog run --branch feature-b --tool aider --prompt "..."
fog list  # See all active tasks
```

### Team Collaboration
```
# Slack: Start task
/fog create branch feature-api and add REST endpoints

# Get notification when done
✅ Task completed
[Open Branch] [Create PR]
```

### CI/CD Integration
```bash
# Via API
curl -X POST http://localhost:8080/api/tasks/create \
  -d '{"repo":"acme-api","branch":"fix","prompt":"Fix bug","ai_tool":"claude"}'
# ai_tool can be omitted only when default_tool is configured
```

## 🔧 AI Tool Support

| Tool | Status | CLI | Notes |
|------|--------|-----|-------|
| Claude Code | ✅ | Yes | Full support |
| Aider | ✅ | Yes | Full support |
| Cursor | ✅ | Yes | Headless via `cursor-agent` |

Adding new tools: Implement `ai.Tool` interface in `internal/ai/`

## 🌐 HTTP API

### Endpoints

```bash
# Health check
GET /health

# List tasks
GET /api/tasks

# Get task
GET /api/tasks/{id}

# Create task
POST /api/tasks/create
{
  "branch": "feature-name",
  "repo": "acme-api",
  "prompt": "Task description",
  "ai_tool": "claude",
  "options": {
    "commit": true,
    "async": true
  }
}
```

## 💬 Slack Setup

1. Create app at https://api.slack.com/apps
2. Add slash command `/fog`
3. Point to `https://your-tunnel.ngrok.io/slack/command`
4. Start fogd:
   ```bash
   ngrok http 8080
   fogd --port 8080 --enable-slack --slack-secret <secret>
   ```

## 🏗️ Architecture

```
┌──────────────────────────────────┐
│       User Interfaces            │
│  CLI │ Slack │ API │ VS Code     │
└──────────┬───────────────────────┘
           │
    ┌──────▼──────┐
    │    fogd     │
    │(HTTP + Slack)│
    └──────┬──────┘
           │
    ┌──────▼──────┐
    │ Fog Runner  │
    │(Orchestrate)│
    └──────┬──────┘
           │
    ┌──────▼──────┐
    │ wtx + AI    │
    │  (Execute)  │
    └─────────────┘
```

## 🔄 Task Lifecycle

```
CREATED
  ↓
SETUP (run setup_cmd)
  ↓
AI_RUNNING (invoke AI tool)
  ↓
VALIDATING (run validate_cmd)
  ↓
COMMITTED (git commit)
  ↓
PR_CREATED (gh pr create)
  ↓
COMPLETED | FAILED
```

## 🛡️ Safety Features

- ✅ Worktree isolation - Never touch main
- ✅ Dirty detection - Warns before deleting
- ✅ Atomic operations - No partial writes
- ✅ No force-push - Append-only
- ✅ Failed preservation - Keep for debugging

## 🔌 Extensions

### VS Code
Tree view + quick switcher (Cmd+Shift+W)

```bash
cd plugins/vscode
npm install && npm run package
code --install-extension *.vsix
```

### Claude Code (MCP)
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

## 🚧 Roadmap

- [x] wtx - Worktree management
- [x] fog - AI orchestration  
- [x] fogd - HTTP API
- [x] Slack integration
- [x] VS Code extension
- [x] Claude Code MCP
- [ ] Web GUI
- [ ] PR comment → re-run
- [ ] Docker isolation
- [ ] Team features

## 💻 Development

```bash
# Build all
make all

# Build individually
make wtx
make fog
make fogd

# Test
make test

# Install
make install
```

## 📖 Examples

See [docs/FOG.md](docs/FOG.md) for comprehensive examples.

## 📜 License

MIT

---

**Fog** - Turn your laptop into a personal cloud for AI agents
