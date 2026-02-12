package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// Editor represents a code editor
type Editor interface {
	Name() string
	IsAvailable() bool
	Open(path string, reuse bool) error
}

// Detect finds available editors and returns the best one
func Detect(preferred string) (Editor, error) {
	editors := []Editor{
		&VSCode{},
		&Cursor{},
		&Neovim{},
		&ClaudeCode{},
		&Vim{},
	}
	
	// If preferred editor is specified, try it first
	if preferred != "" {
		for _, ed := range editors {
			if ed.Name() == preferred && ed.IsAvailable() {
				return ed, nil
			}
		}
	}
	
	// Try $EDITOR environment variable
	if editorEnv := os.Getenv("EDITOR"); editorEnv != "" {
		for _, ed := range editors {
			if ed.Name() == editorEnv && ed.IsAvailable() {
				return ed, nil
			}
		}
	}
	
	// Fall back to first available editor
	for _, ed := range editors {
		if ed.IsAvailable() {
			return ed, nil
		}
	}
	
	return nil, fmt.Errorf("no editor found")
}

// GetEditor returns a specific editor by name
func GetEditor(name string) Editor {
	switch name {
	case "vscode", "code":
		return &VSCode{}
	case "cursor":
		return &Cursor{}
	case "neovim", "nvim":
		return &Neovim{}
	case "claudecode", "claude-code":
		return &ClaudeCode{}
	case "vim":
		return &Vim{}
	default:
		return nil
	}
}

// commandExists checks if a command is available
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
