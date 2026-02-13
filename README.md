# Fog - Local AI Agent Orchestration

> "Turn your local machine into cloud agents"

**Domain:** getfog.dev

Fog orchestrates AI coding tasks using existing AI tools in isolated Git worktrees. Safe, local, and async.

## 🎯 What is Fog?

Fog is a **local-first developer system** that:
- Runs AI coding tasks in **isolated worktrees**
- Supports **Cursor, Claude Code, Aider**
- Provides **CLI, Desktop App, and HTTP API** interfaces
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
fog config view       # Combined wtx + fog config view
```

### 3. fogd - Control Plane
Daemon with local HTTP API (desktop app uses this)

```bash
fogd --port 8080
```

## 🚀 Quick Start

### Installation

```bash
# Install all components
make install

# Or via Go
go install github.com/darkLord19/foglet/cmd/{wtx,fog,fogd,fogcloud}@latest

# Linux installer (release artifacts + checksum verify)
scripts/install-linux.sh

# Version-pinned Linux install
scripts/install-linux.sh --version v0.1.0
```

Release tags (`v*`) publish multi-platform archives and a generated Homebrew formula (`wtx.rb`) artifact.

### Basic Usage

```bash
# 0. One-time onboarding (PAT + default tool)
fog setup
# Supports both classic and fine-grained GitHub PATs

# 0b. Optional: inspect or update Fog settings
fog config view
fog config set --branch-prefix team --default-tool claude

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

# 3b. Desktop app (bundles fogd; starts local server if needed)
# requires Wails CLI installed for dev mode
make fogapp-dev
# or launch installed desktop binary wrapper
fog app

# 4. Discover/import repos via configured PAT
fog repos discover
fog repos import
# import registers repo metadata and initializes:
# ~/.fog/repos/<alias>/repo.git (bare clone)
# ~/.fog/repos/<alias>/base (base worktree)
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
- 🧩 **Desktop-first UI** - `fogapp` (Wails) is the primary local UI
- 🔗 **Embedded daemon** - desktop app starts bundled fogd API server when needed
- 🔄 **Async** - Fire-and-forget execution
- 📢 **Notifications** - Completion alerts
- 🔌 **Extensible** - Easy to add integrations
- ✋ **Real stop semantics** - cancels active process group for current run
- 🌳 **Per-run isolation** - every follow-up/re-run gets a new worktree

### Desktop Session UX (current)
- Session title is derived from the first line of the prompt.
- Sidebar separates running and completed sessions.
- Task detail auto-follows the latest run for timeline/logs/diff.
- `Stop` cancels only the latest active run.
- `Re-run` schedules a new run in a separate worktree on the same branch.
- Diff tab shows changes since base branch (`base...session-branch`).
- `Open in Editor` opens the latest run worktree.

## 📚 Documentation

- **[Current State](docs/CURRENT_STATE.md)** - Implemented behavior snapshot
- **[Testing Guide](docs/TESTING.md)** - Automated + end-to-end validation steps
- **[Release Guide](docs/RELEASE.md)** - Artifact packaging, Homebrew formula generation, release workflow
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

# Session APIs (desktop-first)
GET /api/sessions
POST /api/sessions
GET /api/sessions/{id}
GET /api/sessions/{id}/runs
POST /api/sessions/{id}/runs
GET /api/sessions/{id}/runs/{run_id}/events
POST /api/sessions/{id}/cancel
GET /api/sessions/{id}/diff
POST /api/sessions/{id}/open
```

## 🏗️ Architecture

```
┌──────────────────────────────────┐
│       User Interfaces            │
│ CLI │ Desktop │ API │ VS Code │
└──────────┬───────────────────────┘
           │
    ┌──────▼──────┐
    │    fogd     │
    │  (HTTP API)  │
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
COMPLETED | FAILED | CANCELLED
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
- [x] Desktop app (Wails)
- [x] VS Code extension
- [x] Claude Code MCP
- [ ] Advanced GUI workflows
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
For validation steps, see [docs/TESTING.md](docs/TESTING.md).

## 📜 License

AGPL-3.0-or-later

---

**Fog** - Turn your laptop into a personal cloud for AI agents
