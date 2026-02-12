package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ListWorktrees returns all worktrees in the repository
func (g *Git) ListWorktrees() ([]Worktree, error) {
	output, err := g.exec("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	
	return parseWorktreeList(output), nil
}

// AddWorktree creates a new worktree
func (g *Git) AddWorktree(path, branch string) error {
	_, err := g.exec("worktree", "add", path, branch)
	return err
}

// AddWorktreeNewBranch creates a new worktree with a new branch
func (g *Git) AddWorktreeNewBranch(path, branch, startPoint string) error {
	_, err := g.exec("worktree", "add", "-b", branch, path, startPoint)
	return err
}

// RemoveWorktree removes a worktree
func (g *Git) RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	
	_, err := g.exec(args...)
	return err
}

// PruneWorktrees removes worktree information for deleted worktrees
func (g *Git) PruneWorktrees(dryRun bool) ([]string, error) {
	args := []string{"worktree", "prune", "--verbose"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	
	output, err := g.exec(args...)
	if err != nil {
		return nil, err
	}
	
	var pruned []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			pruned = append(pruned, line)
		}
	}
	
	return pruned, nil
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	var current *Worktree
	
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}
		
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		
		key, value := parts[0], parts[1]
		
		switch key {
		case "worktree":
			current = &Worktree{
				Path: value,
				Name: filepath.Base(value),
			}
		case "HEAD":
			if current != nil {
				current.Head = value
			}
		case "branch":
			if current != nil {
				// branch refs/heads/main -> main
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "locked":
			if current != nil {
				current.Locked = true
			}
		case "prunable":
			if current != nil {
				current.Prunable = true
			}
		}
	}
	
	// Don't forget the last one
	if current != nil {
		worktrees = append(worktrees, *current)
	}
	
	return worktrees
}
