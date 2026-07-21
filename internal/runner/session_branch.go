package runner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/branchname"
	"github.com/darkLord19/foglet/internal/git"
)

// ResolveBranch resolves a unique branch name for a session.
// If requested is non-empty, it validates and returns it.
// Otherwise, it generates a slug from the prompt and ensures uniqueness.
func (r *Runner) ResolveBranch(repoPath, requested, prompt string) (string, error) {
	return branchname.Resolve(requested, r.branchPrefix(), prompt, git.New(repoPath).BranchExists)
}

// branchPrefix returns the configured branch prefix, or "" to accept the default.
func (r *Runner) branchPrefix() string {
	if r == nil || r.state == nil {
		return ""
	}
	stored, found, err := r.state.GetSetting("branch_prefix")
	if err != nil || !found {
		return ""
	}
	return strings.TrimSpace(stored)
}

// SessionDiff returns the diff stat and diff patch for a session's branch
// against its base branch.
func (r *Runner) SessionDiff(sessionID string) (diffStat, diffPatch string, err error) {
	if r.state == nil {
		return "", "", errors.New("state store not configured")
	}
	session, found, err := r.state.GetSession(sessionID)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", fmt.Errorf("session %q not found", sessionID)
	}

	repo, found, err := r.state.GetRepoByName(session.RepoName)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", fmt.Errorf("repo %q not found", session.RepoName)
	}

	worktreePath := strings.TrimSpace(session.WorktreePath)
	if latest, found, err := r.state.GetLatestRun(session.ID); err == nil && found && strings.TrimSpace(latest.WorktreePath) != "" {
		worktreePath = strings.TrimSpace(latest.WorktreePath)
	}
	if worktreePath == "" {
		return "", "", errors.New("session has no worktree path")
	}

	baseBranch := strings.TrimSpace(repo.DefaultBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}

	g := git.New(worktreePath)
	diffRef := fmt.Sprintf("%s...%s", baseBranch, session.Branch)

	stat, err := g.DiffStat(diffRef)
	if err != nil {
		return "", "", fmt.Errorf("git diff stat: %w", err)
	}

	patch, err := g.Diff(diffRef)
	if err != nil {
		return "", "", fmt.Errorf("git diff: %w", err)
	}

	return strings.TrimSpace(stat), strings.TrimSpace(patch), nil
}
