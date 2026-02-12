# Fog - Local AI Agent Orchestration

**Turn your local machine into cloud agents**

Fog orchestrates AI coding tasks using existing AI tools (Cursor, Claude Code, Aider), executes them in isolated Git worktrees, and exposes async control via CLI, HTTP API, and Slack.

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
fog run \
  --branch feature-otp \
  --tool claude \
  --prompt "Add OTP login using Redis" \
  --commit \
  --pr

fog list              # List all tasks
fog status <id>       # Task status
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
fogd --enable-slack \            # With Slack
     --slack-secret <secret>
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

### 2. Run AI Task with fog

```bash
fog run \
  --branch feature-auth \
  --tool claude \
  --prompt "Implement JWT authentication" \
  --setup-cmd "npm install" \
  --validate-cmd "npm test" \
  --commit
```

### 3. Start fogd (Optional)

```bash
# Start daemon
fogd --port 8080

# In another terminal, test API
curl http://localhost:8080/api/tasks

# Create task via API
curl -X POST http://localhost:8080/api/tasks/create \
  -H "Content-Type: application/json" \
  -d '{
    "branch": "feature-api",
    "prompt": "Add REST API for users",
    "ai_tool": "claude",
    "options": {
      "commit": true,
      "async": true
    }
  }'
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

3. **Cursor** - Opens project (no CLI yet)
   ```bash
   fog run --tool cursor --prompt "..."
   # Opens in Cursor for manual work
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

See `pkg/fog/ai/` for examples.

## Slack Integration

### Setup

1. Create Slack App at https://api.slack.com/apps

2. Add Slash Command:
   - Command: `/fog`
   - Request URL: `https://your-tunnel.ngrok.io/slack/command`
   - Description: `Run AI coding tasks`

3. Install to workspace

4. Start fogd with Slack:
   ```bash
   # Use ngrok or cloudflared for tunnel
   ngrok http 8080

   # Start fogd
   fogd --port 8080 \
        --enable-slack \
        --slack-secret <your-signing-secret>
   ```

### Usage

In Slack:
```
/fog create branch feature-login and implement OAuth login
```

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

**POST /api/tasks/create**
```bash
curl -X POST http://localhost:8080/api/tasks/create \
  -H "Content-Type: application/json" \
  -d '{
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

**GET /api/tasks/{id}**
```bash
curl http://localhost:8080/api/tasks/<task-id>
```

### Response Format

```json
{
  "id": "uuid",
  "state": "COMPLETED",
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
- After PAT is saved, Fog can list accessible GitHub repos for users to select/import

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
  --branch feature-user-api \
  --tool claude \
  --prompt "Add REST API endpoints for user CRUD" \
  --commit \
  --pr
```

### Example 2: With Validation

```bash
fog run \
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

```
/fog create branch feature-search and implement full-text search with Elasticsearch
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
├── internal/       # wtx internals
│   ├── git/
│   ├── tui/
│   ├── editor/
│   ├── metadata/
│   └── config/
├── pkg/fog/        # Fog shared packages
│   ├── task/       # Task lifecycle
│   ├── ai/         # AI tool adapters
│   ├── runner/     # Orchestration engine
│   ├── api/        # HTTP API
│   └── slack/      # Slack integration
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
