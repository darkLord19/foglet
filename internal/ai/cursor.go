package ai

import (
	"fmt"
	"os/exec"
	"strings"
)

// Cursor represents the Cursor AI tool.
type Cursor struct{}

func (c *Cursor) Name() string {
	return "cursor"
}

func (c *Cursor) IsAvailable() bool {
	return cursorAgentCommand() != ""
}

func (c *Cursor) Execute(workdir, prompt string) (*Result, error) {
	cmdName := cursorAgentCommand()
	if cmdName == "" {
		return nil, fmt.Errorf("cursor-agent not available")
	}

	output, err := runCursorHeadless(cmdName, workdir, prompt)
	if err != nil {
		return &Result{
			Success: false,
			Output:  strings.TrimSpace(string(output)),
			Error:   err,
		}, err
	}

	return &Result{
		Success: true,
		Output:  strings.TrimSpace(string(output)),
	}, nil
}

func cursorAgentCommand() string {
	if commandExists("cursor-agent") {
		return "cursor-agent"
	}
	return ""
}

func runCursorHeadless(cmdName, workdir, prompt string) ([]byte, error) {
	primary := buildCursorHeadlessArgs(prompt, true)
	output, err := runCursorCommand(cmdName, workdir, primary)
	if err == nil {
		return output, nil
	}

	combined := string(output)
	if strings.Contains(combined, "unknown flag") || strings.Contains(combined, "flag provided but not defined") {
		fallback := buildCursorHeadlessArgs(prompt, false)
		return runCursorCommand(cmdName, workdir, fallback)
	}

	return output, err
}

func runCursorCommand(cmdName, workdir string, args []string) ([]byte, error) {
	cmd := exec.Command(cmdName, args...)
	cmd.Dir = workdir
	return cmd.CombinedOutput()
}

func buildCursorHeadlessArgs(prompt string, withOutputFormat bool) []string {
	args := []string{"-p", "--force"}
	if withOutputFormat {
		args = append(args, "--output-format", "text")
	}
	args = append(args, prompt)
	return args
}
