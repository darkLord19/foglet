# Fog - Local AI Agent Orchestration

**Turn your local machine into cloud agents**

Fog orchestrates AI coding tasks using existing AI tools (Cursor, Claude Code, Aider), executes them in isolated Git worktrees, and exposes async control via CLI, HTTP API, and Slack.

Supporting docs:
- Current implementation snapshot: `docs/CURRENT_STATE.md`
- End-to-end validation steps: `docs/TESTING.md`
- Release and Homebrew packaging: `docs/RELEASE.md`

## Architecture

Fog consists of three main components:

### 1. wtx - Worktree CLI (Foundation)
Pure Git worktree management with zero AI, zero networking.

**Responsibilities:**
- Create, list, remove worktrees
- Open worktrees in editors
- Run setup hooks (deps install)
- Track metadata

**Commands:**
```bash
wtx                    # Interactive TUI
wtx list              # List worktrees
wtx add <n>          # Create worktree
wtx add --json <n>   # Create worktree with JSON output (for automation)
wtx open <n>         # Open in editor
wtx rm <n>           # Remove worktree
wtx status <n>       # Detailed status
wtx config           # View configuration
```

### 2. fog - AI Orchestration CLI
Execute AI tasks safely and locally.

**Responsibilities:**
- Uses wtx for isolation
- Invokes AI CLIs (Cursor, Claude Code, Aider)
- Tracks task lifecycle
- Optional validation, commit, PR creation

**Commands:**
```bash
fog setup             # Configure PAT + default tool
fog run \
  --branch feature-otp \
  --tool claude \
  --prompt "Add OTP login using Redis" \
  --commit \
  --pr

fog list              # List all tasks
fog status <id>       # Task status
fog config view       # Combined wtx + fog config
fog config set --branch-prefix team --default-tool claude
fog repos discover    # List repos accessible by PAT
fog repos import      # Select and register repos
fog repos list        # List registered repos
```

### 3. fogd - Control Plane Daemon
Async control + integrations.

**Responsibilities:**
- HTTP API server
- Receives tasks from Slack/web
- Queues and executes via fog runner
- Sends notifications
- Manages tunnels (for Slack)

**Usage:**
```bash
fogd --port 8080                  # API only
fogd --enable-slack --slack-mode http \
     --slack-secret <secret>      # HTTP slash-command mode
fogd --enable-slack --slack-mode socket \
     --slack-bot-token <xoxb-...> \
     --slack-app-token <xapp-...> # Socket Mode (@fog mentions)
```

## Installation

```bash
# Install all components
make install

# Or install individually
go install github.com/darkLord19/wtx/cmd/wtx@latest
go install github.com/darkLord19/wtx/cmd/fog@latest
go install github.com/darkLord19/wtx/cmd/fogd@latest
```

## Quick Start

### 1. Setup wtx

```bash
# Create config
mkdir -p ~/.config/wtx
cat > ~/.config/wtx/config.json << EOF
{
  "editor": "cursor",
  "reuse_window": true,
  "worktree_dir": "../worktrees",
  "setup_cmd": "pnpm install",
  "validate_cmd": "pnpm test",
  "default_branch": "main"
}
EOF

# Try it out
wtx list
wtx add test-branch
```

### 2. Onboard Fog (one-time)

```bash
fog setup
```

### 3. Run AI Task with fog

```bash
fog run \
  --repo acme-api \
  --branch feature-auth \
  --tool claude \
  --prompt "Implement JWT authentication" \
  --setup-cmd "npm install" \
  --validate-cmd "npm test" \
  --commit
# --tool is optional once default_tool is configured via `fog setup`
```

### 4. Discover/import repositories

```bash
fog repos discover
fog repos import
```

### 5. Start fogd (Optional)

```bash
# Start daemon
fogd --port 8080

# In another terminal, test API
curl http://localhost:8080/api/tasks

# Create task via API
curl -X POST http://localhost:8080/api/tasks/create \
  -H "Content-Type: application/json" \
  -d '{
    "repo": "acme-api",
    "branch": "feature-api",
    "prompt": "Add REST API for users",
    "ai_tool": "claude",
    "options": {
      "commit": true,
      "async": true
    }
  }'
# ai_tool can be omitted only when default_tool exists
```

## Task Lifecycle

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
COMPLETED
```

## AI Tool Support

### Supported Tools

1. **Claude Code** - Full CLI support
   ```bash
   fog run --tool claude --prompt "..."
   ```

2. **Aider** - Full CLI support
   ```bash
   fog run --tool aider --prompt "..."
   ```

3. **Cursor** - Headless CLI support
   ```bash
   fog run --tool cursor --prompt "..."
   # Uses cursor-agent headless mode
   ```

### Adding New Tools

Implement the `ai.Tool` interface:

```go
type Tool interface {
    Name() string
    IsAvailable() bool
    Execute(workdir, prompt string) (*Result, error)
}
```

See `internal/ai/` for examples.

## Slack Integration

### Setup

1. Create Slack App at https://api.slack.com/apps

2. Enable Socket Mode and create an app token (`xapp-...`) with `connections:write`

3. Add bot scopes and install app:
   - `chat:write`
   - `app_mentions:read`
   - `commands`
   - Install app to workspace and copy bot token (`xoxb-...`)

4. Start fogd with Slack Socket Mode:
   ```bash
   fogd --port 8080 \
        --enable-slack \
        --slack-mode socket \
        --slack-bot-token <your-bot-token> \
        --slack-app-token <your-app-token>
   ```

5. Optional: HTTP slash-command mode is still available:
   ```bash
   ngrok http 8080
   fogd --port 8080 \
        --enable-slack \
        --slack-mode http \
        --slack-secret <your-signing-secret>
   ```

### Usage

In Slack:
```
@fog [repo='acme-api' tool='claude' autopr=true branch-name='feature-login' commit-msg='add oauth login'] implement OAuth login

# Follow-up in same thread (plain prompt only)
@fog tighten validation and add tests
```

Rules:
- `repo` is required.
- Optional keys: `tool`, `model`, `autopr`, `branch-name`, `commit-msg`.
- If `branch-name` is omitted, Fog generates a branch from prompt and `branch_prefix`.
- Unknown keys are rejected.
- Thread follow-ups must be plain prompts (no options block). Follow-ups reuse repo/tool context from the latest Fog task in that thread.

Response:
```
🚀 Starting task on branch `feature-login`
implement OAuth login

[✅ after completion]
✅ Task completed: feature-login
Branch: feature-login
Duration: 2m 30s

[Open Branch] [Create PR]
```

## HTTP API

### Endpoints

**GET /health**
```bash
curl http://localhost:8080/health
```

**GET /api/tasks**
```bash
curl http://localhost:8080/api/tasks
```

**GET /api/settings**
```bash
curl http://localhost:8080/api/settings
```

**PUT /api/settings**
```bash
curl -X PUT http://localhost:8080/api/settings \
  -H "Content-Type: application/json" \
  -d '{
    "default_tool": "claude",
    "branch_prefix": "fog"
  }'
```

**POST /api/tasks/create**
```bash
curl -X POST http://localhost:8080/api/tasks/create \
  -H "Content-Type: application/json" \
  -d '{
    "repo": "acme-api",
    "branch": "feature-name",
    "prompt": "Your task description",
    "ai_tool": "claude",
    "options": {
      "commit": true,
      "create_pr": false,
      "validate": true,
      "async": true
}
  }'
```

### 6. Open Web UI

```bash
fog ui
# Checks /health; if fogd is not running, starts it and opens browser.
# The page is served by fogd at GET / and shows:
# - active/all task list (auto-refresh)
# - default tool + branch prefix settings editor
```

**GET /api/tasks/{id}**
```bash
curl http://localhost:8080/api/tasks/<task-id>
```

### Response Format

```json
{
  "id": "uuid",
  "state": "COMPLETED",
  "repo": "acme-api",
  "branch": "feature-auth",
  "prompt": "Implement authentication",
  "ai_tool": "claude",
  "worktree_path": "/path/to/worktree",
  "created_at": "2024-02-12T10:00:00Z",
  "updated_at": "2024-02-12T10:05:00Z",
  "completed_at": "2024-02-12T10:05:00Z",
  "metadata": {
    "pr_url": "https://github.com/..."
  }
}
```

## Configuration

### wtx Config (~/.config/wtx/config.json)

```json
{
  "editor": "cursor",
  "reuse_window": true,
  "worktree_dir": "../worktrees",
  "auto_start_dev": false,
  "default_branch": "main",
  "setup_cmd": "pnpm install",
  "validate_cmd": "pnpm test"
}
```

### Fog State (`~/.fog/`)

- SQLite database: `~/.fog/fog.db`
- Master encryption key (local file): `~/.fog/master.key`
- GitHub PAT: encrypted at rest in SQLite (AES-GCM)
- Managed repo registry: stored in SQLite (`repos` table)

Notes:
- PAT is persisted as encrypted ciphertext only (no raw token on disk).
- The local master key is file-based (no OS keychain dependency).

### Onboarding (v1)

- Required: GitHub PAT
- Required: default AI tool selection
- Supports both classic and fine-grained GitHub PATs
- After PAT is saved, Fog can list accessible GitHub repos for users to select/import
- Import initializes managed repo layout:
  - `~/.fog/repos/<alias>/repo.git` (bare clone)
  - `~/.fog/repos/<alias>/base` (base worktree)

## Safety Rules

1. **Always create new worktree** - Never run AI in main branch
2. **Never force-push** - All operations are append-only
3. **Never delete dirty worktrees** - Requires confirmation
4. **Preserve failed worktrees** - For inspection
5. **Atomic operations** - All file writes are atomic

## PR Creation

Uses GitHub CLI (`gh`):
- Requires `gh auth login`
- Deterministic title: `feat: {prompt}`
- Templated body with task metadata
- If PAT-based repo features are used, Fog stores an encrypted PAT locally

## Examples

### Example 1: Simple Feature

```bash
fog run \
  --repo acme-api \
  --branch feature-user-api \
  --tool claude \
  --prompt "Add REST API endpoints for user CRUD" \
  --commit \
  --pr
```

### Example 2: With Validation

```bash
fog run \
  --repo acme-api \
  --branch fix-auth-bug \
  --tool aider \
  --prompt "Fix JWT token expiration bug" \
  --setup-cmd "npm ci" \
  --validate-cmd "npm test" \
  --commit
```

### Example 3: Async Execution

```bash
fog run \
  --repo acme-api \
  --branch refactor-db \
  --tool claude \
  --prompt "Migrate database from MongoDB to PostgreSQL" \
  --async

# Check status later
fog list
fog status <task-id>
```

### Example 4: Via API

```bash
curl -X POST http://localhost:8080/api/tasks/create \
  -H "Content-Type: application/json" \
  -d '{
    "repo": "acme-api",
    "branch": "feature-notifications",
    "prompt": "Add email notification system",
    "ai_tool": "claude",
    "options": {
      "setup_cmd": "npm install",
      "validate_cmd": "npm test",
      "commit": true,
      "create_pr": true,
      "async": true
    }
  }'
```

### Example 5: Via Slack

```text
/fog [repo='acme-api' tool='claude'] implement full-text search with Elasticsearch
```

## Troubleshooting

### AI Tool Not Found

```bash
# Check availability
which claude
which aider
which cursor

# Install Claude Code
npm install -g @anthropic-ai/claude-code

# Install Aider
pip install aider-chat
```

### Worktree Creation Failed

```bash
# Check wtx
wtx list
wtx config

# Verify git repo
git worktree list
```

### fogd Not Starting

```bash
# Check port availability
lsof -i :8080

# Check logs
fogd --port 8080  # Run in foreground
```

### Slack Not Responding

```bash
# Verify tunnel
curl http://localhost:8080/slack/command

# Check ngrok status
ngrok http 8080

# Verify Slack app config
# Request URL should match ngrok URL
```

## Development

### Build All

```bash
make build
# or
make all
```

### Build Individually

```bash
make wtx
make fog
make fogd
```

### Run Tests

```bash
make test
```

### Project Structure

```
.
├── cmd/
│   ├── wtx/        # Worktree CLI
│   ├── fog/        # AI orchestration CLI
│   └── fogd/       # Daemon
├── internal/
│   ├── git/
│   ├── tui/
│   ├── editor/
│   ├── metadata/
│   ├── config/
│   ├── task/       # Fog task lifecycle
│   ├── ai/         # Fog AI tool adapters
│   ├── runner/     # Fog orchestration engine
│   ├── api/        # Fog HTTP API
│   ├── slack/      # Fog Slack integration
│   ├── state/      # Fog sqlite + encrypted secrets
│   ├── github/     # GitHub API client
│   └── env/        # Fog home/path helpers
└── plugins/
    ├── vscode/     # VS Code extension
    └── claude-code/# Claude Code MCP
```

## Roadmap

- [x] wtx - Worktree management
- [x] fog - AI orchestration
- [x] fogd - HTTP API
- [x] Slack integration
- [ ] GUI web interface
- [ ] PR comment → re-run loop
- [ ] More AI tool adapters
- [ ] Docker/container isolation
- [ ] Team collaboration features

## License

MIT

---

**Fog - Turn your laptop into a personal cloud for AI agents**
