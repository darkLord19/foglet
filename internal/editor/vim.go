package editor

import (
	"os"
	"os/exec"
)

// Vim represents Vim editor
type Vim struct{}

func (v *Vim) Name() string {
	return "vim"
}

func (v *Vim) IsAvailable() bool {
	return commandExists("vim")
}

func (v *Vim) Open(path string, reuse bool) error {
	// Vim runs in the terminal
	cmd := exec.Command("vim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = path

	return cmd.Run()
}
