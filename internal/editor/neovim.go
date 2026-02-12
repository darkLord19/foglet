package editor

import (
	"os"
	"os/exec"
)

// Neovim represents Neovim editor
type Neovim struct{}

func (n *Neovim) Name() string {
	return "neovim"
}

func (n *Neovim) IsAvailable() bool {
	return commandExists("nvim")
}

func (n *Neovim) Open(path string, reuse bool) error {
	// Neovim runs in the terminal, so we need to change directory
	// and launch it
	cmd := exec.Command("nvim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = path

	return cmd.Run()
}
