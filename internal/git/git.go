package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Git wraps git operations for a repository
type Git struct {
	repoPath string
}

// New creates a new Git instance for the given repository path
func New(repoPath string) *Git {
	return &Git{repoPath: repoPath}
}

// Worktree represents a git worktree
type Worktree struct {
	Name     string
	Path     string
	Branch   string
	Head     string
	Locked   bool
	Prunable bool
}

// Status represents the git status of a worktree
type Status struct {
	Clean  bool
	Dirty  bool
	Ahead  int
	Behind int
	Stash  bool
}

// Remote represents remote tracking information
type Remote struct {
	Name   string
	Branch string
	URL    string
}

// exec runs a git command and returns the output
func (g *Git) exec(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoPath
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	
	return strings.TrimSpace(string(output)), nil
}

// IsRepo checks if the current directory is a git repository
func (g *Git) IsRepo() bool {
	_, err := g.exec("rev-parse", "--git-dir")
	return err == nil
}

// GetRepoRoot returns the root directory of the repository
func (g *Git) GetRepoRoot() (string, error) {
	return g.exec("rev-parse", "--show-toplevel")
}
