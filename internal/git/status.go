package git

import (
	"strconv"
	"strings"
)

// GetStatus returns the git status for a worktree
func (g *Git) GetStatus(worktreePath string) (*Status, error) {
	// Create a new Git instance for the worktree
	wtGit := New(worktreePath)

	status := &Status{
		Clean: true,
	}

	// Check if working tree is clean
	output, err := wtGit.exec("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	if output != "" {
		status.Clean = false
		status.Dirty = true
	}

	// Check ahead/behind status
	output, err = wtGit.exec("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		// Output format: "ahead\tbehind"
		parts := strings.Split(output, "\t")
		if len(parts) == 2 {
			status.Ahead, _ = strconv.Atoi(parts[0])
			status.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	// Check for stashes
	output, err = wtGit.exec("stash", "list")
	if err == nil && output != "" {
		status.Stash = true
	}

	return status, nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func (g *Git) HasUncommittedChanges(worktreePath string) (bool, error) {
	wtGit := New(worktreePath)
	output, err := wtGit.exec("status", "--porcelain")
	if err != nil {
		return false, err
	}

	return output != "", nil
}

// GetBranch returns the current branch name
func (g *Git) GetBranch(worktreePath string) (string, error) {
	wtGit := New(worktreePath)
	output, err := wtGit.exec("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}

	return output, nil
}

// GetRemote returns remote tracking information
func (g *Git) GetRemote(worktreePath string) (*Remote, error) {
	wtGit := New(worktreePath)

	// Get remote name
	remoteName, err := wtGit.exec("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return nil, err
	}

	// Parse remote/branch
	parts := strings.SplitN(remoteName, "/", 2)
	if len(parts) != 2 {
		return nil, nil
	}

	remote := &Remote{
		Name:   parts[0],
		Branch: parts[1],
	}

	// Get remote URL
	url, err := wtGit.exec("remote", "get-url", remote.Name)
	if err == nil {
		remote.URL = url
	}

	return remote, nil
}
