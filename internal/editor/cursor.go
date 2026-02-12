package editor

import (
	"os/exec"
)

// Cursor represents Cursor editor
type Cursor struct{}

func (c *Cursor) Name() string {
	return "cursor"
}

func (c *Cursor) IsAvailable() bool {
	return commandExists("cursor")
}

func (c *Cursor) Open(path string, reuse bool) error {
	args := []string{path}
	if reuse {
		args = append([]string{"-r"}, args...)
	}
	
	cmd := exec.Command("cursor", args...)
	return cmd.Start()
}
