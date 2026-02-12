#!/bin/bash

set -e

echo "Installing wtx + fog + fogd..."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Install binaries
echo -e "${YELLOW}Installing binaries...${NC}"
go install github.com/darkLord19/wtx/cmd/wtx@latest
go install github.com/darkLord19/wtx/cmd/fog@latest
go install github.com/darkLord19/wtx/cmd/fogd@latest

# Check if installation was successful
if command -v wtx &> /dev/null && command -v fog &> /dev/null && command -v fogd &> /dev/null; then
    echo -e "${GREEN}✓ wtx/fog/fogd installed successfully${NC}"
    wtx version || true
    fog version || true
    fogd version || true
else
    echo -e "${RED}Error: Installation failed${NC}"
    echo "Make sure \$GOPATH/bin is in your PATH"
    exit 1
fi

# Install shell completions
echo ""
echo -e "${YELLOW}Installing shell completions...${NC}"

# Detect shell
SHELL_NAME=$(basename "$SHELL")

case "$SHELL_NAME" in
    bash)
        COMPLETION_DIR="$HOME/.local/share/bash-completion/completions"
        mkdir -p "$COMPLETION_DIR"
        curl -sL https://raw.githubusercontent.com/darkLord19/wtx/main/scripts/completions/wtx.bash \
            -o "$COMPLETION_DIR/wtx"
        echo -e "${GREEN}✓ Bash completions installed${NC}"
        echo "Run: source $COMPLETION_DIR/wtx"
        ;;
    zsh)
        COMPLETION_DIR="${ZDOTDIR:-$HOME}/.zsh/completions"
        mkdir -p "$COMPLETION_DIR"
        curl -sL https://raw.githubusercontent.com/darkLord19/wtx/main/scripts/completions/wtx.zsh \
            -o "$COMPLETION_DIR/_wtx"
        echo -e "${GREEN}✓ Zsh completions installed${NC}"
        echo "Add to ~/.zshrc: fpath=($COMPLETION_DIR \$fpath)"
        ;;
    fish)
        echo -e "${YELLOW}Fish completions not yet available${NC}"
        ;;
    *)
        echo -e "${YELLOW}Unknown shell, skipping completions${NC}"
        ;;
esac

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Quick start:"
echo "  wtx              # Launch interactive UI"
echo "  wtx list         # List worktrees"
echo "  wtx add <n>     # Create worktree"
echo "  wtx open <n>    # Open in editor"
echo "  fog setup        # Configure PAT + default tool"
echo "  fog ui           # Open Fog web UI (auto-starts fogd)"
echo "  fog run --help   # Run AI task in isolated worktree"
echo ""
echo "For more info: wtx --help, fog --help, fogd --help"
