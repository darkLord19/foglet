FROM debian:bookworm-slim

ARG CLAUDE_VERSION
LABEL org.opencontainers.image.title="fog-claude" \
      org.opencontainers.image.description="Claude Code CLI for Fog VM sandbox" \
      org.opencontainers.image.source="https://github.com/darkLord19/foglet" \
      fog.claude.version="${CLAUDE_VERSION}"

# Install Node.js LTS + git. Node is required by the Claude CLI (@anthropic-ai/claude-code).
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl gnupg git \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install the Claude Code CLI globally. Pin the version so the image label
# and the installed binary always agree.
RUN npm install -g "@anthropic-ai/claude-code@${CLAUDE_VERSION}" \
    && claude --version

# The microsandbox runtime mounts the host worktree at /workspace.
WORKDIR /workspace
