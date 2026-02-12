package editor

import (
	"os/exec"
)

// VSCode represents Visual Studio Code
type VSCode struct{}

func (v *VSCode) Name() string {
	return "vscode"
}

func (v *VSCode) IsAvailable() bool {
	return commandExists("code")
}

func (v *VSCode) Open(path string, reuse bool) error {
	args := []string{path}
	if reuse {
		args = append([]string{"-r"}, args...)
	}
	
	cmd := exec.Command("code", args...)
	return cmd.Start()
}
