package git

import (
	"fmt"
	"strings"
)

// The git operations a session run performs. These previously lived in
// internal/runner as direct proc.Run invocations, which meant Fog had two ways
// to run git: this module (uncancellable, via exec.Command) and raw proc.Run
// calls scattered through the runner. Consolidating them here leaves one path,
// and every caller inside a run binds a context via WithContext.

// IsDirty reports whether the worktree has uncommitted changes.
func (g *Git) IsDirty() (bool, error) {
	output, err := g.exec("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

// StageAll stages every change in the worktree.
func (g *Git) StageAll() error {
	_, err := g.exec("add", ".")
	return err
}

// Commit records the staged changes with the given message and returns the new
// commit SHA.
func (g *Git) Commit(message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}
	if _, err := g.exec("commit", "-m", message); err != nil {
		return "", err
	}
	return g.HeadSHA()
}

// HeadSHA returns the commit SHA at HEAD.
func (g *Git) HeadSHA() (string, error) {
	return g.exec("rev-parse", "HEAD")
}

// Push pushes branch to origin. When setUpstream is true the branch is
// configured to track the remote.
func (g *Git) Push(branch string, setUpstream bool) error {
	args := []string{"push", "origin", branch}
	if setUpstream {
		args = []string{"push", "-u", "origin", branch}
	}
	_, err := g.exec(args...)
	return err
}

// StagedDiff describes the staged changes: a name/status list, a stat summary,
// and the patch itself. The patch is returned whole; callers decide how much of
// it to keep.
type StagedDiff struct {
	NameStatus string
	Stat       string
	Patch      string
}

// StagedChanges returns the diff of everything currently staged.
func (g *Git) StagedChanges() (StagedDiff, error) {
	nameStatus, err := g.exec("diff", "--cached", "--name-status")
	if err != nil {
		return StagedDiff{}, fmt.Errorf("git diff --name-status failed: %w", err)
	}
	stat, err := g.exec("diff", "--cached", "--stat")
	if err != nil {
		return StagedDiff{}, fmt.Errorf("git diff --stat failed: %w", err)
	}
	patch, err := g.exec("diff", "--cached", "--no-color")
	if err != nil {
		return StagedDiff{}, fmt.Errorf("git diff failed: %w", err)
	}
	return StagedDiff{NameStatus: nameStatus, Stat: stat, Patch: patch}, nil
}
