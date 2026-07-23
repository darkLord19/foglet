package runner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/git"
)

// RemoveSessionArtifacts tears down the git worktree and branch a session owns.
//
// It is the destructive half of purging a trashed task: the session record and
// its run history stay in the database, but the on-disk worktree and the unmerged
// branch are reclaimed. Removal is forced — a trashed session's work was never
// merged, so the ordinary "unmerged commits" and "dirty worktree" guards would
// otherwise refuse. The worktree is removed before the branch because git will
// not delete a branch that is still checked out in a live worktree.
//
// Missing artifacts are not an error: a session whose worktree was already
// cleaned up (or never created) should still purge cleanly.
func (r *Runner) RemoveSessionArtifacts(sessionID string) error {
	if r.runs == nil || r.repos == nil {
		return errors.New("state store not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session id is required")
	}

	session, found, err := r.runs.GetSession(sessionID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	repo, found, err := r.repos.GetRepoByName(session.RepoName)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(repo.BaseWorktreePath) == "" {
		return nil
	}

	g := git.New(repo.BaseWorktreePath)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository: %s", repo.BaseWorktreePath)
	}

	var errs []error
	if wt := strings.TrimSpace(session.WorktreePath); wt != "" {
		if err := g.RemoveWorktree(wt, true); err != nil {
			errs = append(errs, fmt.Errorf("remove worktree %s: %w", wt, err))
		}
	}
	if branch := strings.TrimSpace(session.Branch); branch != "" {
		if err := g.DeleteBranch(branch, true); err != nil {
			errs = append(errs, fmt.Errorf("delete branch %s: %w", branch, err))
		}
	}
	return errors.Join(errs...)
}
