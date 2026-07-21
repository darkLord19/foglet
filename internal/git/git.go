package git

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/darkLord19/foglet/internal/proc"
)

// Git wraps git operations for a repository.
//
// Commands run through internal/proc, so they inherit its process-group
// handling and SIGTERM-then-SIGKILL cancellation. A Git carries the context its
// commands run under; use WithContext to bind one. Without it commands are
// uncancellable, which is only appropriate for short-lived queries.
type Git struct {
	repoPath string
	ctx      context.Context
}

// New creates a new Git instance for the given repository path.
//
// The returned Git runs commands under context.Background(). Callers on a
// cancellable path — anything inside a session run — should use WithContext so
// a cancelled run does not leave git processes behind.
func New(repoPath string) *Git {
	return &Git{repoPath: repoPath, ctx: context.Background()}
}

// WithContext returns a copy of g whose commands run under ctx.
func (g *Git) WithContext(ctx context.Context) *Git {
	if ctx == nil {
		ctx = context.Background()
	}
	clone := *g
	clone.ctx = ctx
	return &clone
}

// context returns the bound context, tolerating a zero-value Git.
func (g *Git) context() context.Context {
	if g.ctx == nil {
		return context.Background()
	}
	return g.ctx
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

// exec runs a git command and returns its trimmed combined output.
func (g *Git) exec(args ...string) (string, error) {
	output, err := proc.Run(g.context(), g.repoPath, "git", args...)
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

// CommonDir returns the repository's common git directory, resolved to an
// absolute path. For a worktree this is the main repository's .git directory
// rather than the worktree's own.
func (g *Git) CommonDir() (string, error) {
	gitDir, err := g.exec("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if gitDir == "" {
		return "", fmt.Errorf("resolve git common dir: empty output")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(g.repoPath, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

// BranchExists checks if a local branch exists.
func (g *Git) BranchExists(branch string) bool {
	if strings.TrimSpace(branch) == "" {
		return false
	}
	_, err := g.exec("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// ListBranches returns a list of local branches.
func (g *Git) ListBranches() ([]string, error) {
	out, err := g.exec("branch", "--list", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var branches []string
	for line := range strings.SplitSeq(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			branches = append(branches, trimmed)
		}
	}
	return branches, nil
}

// Diff returns the full patch for a diff reference (e.g. "main...feature").
func (g *Git) Diff(ref string) (string, error) {
	return g.exec("diff", "--no-color", ref)
}

// DiffStat returns the diff stat for a diff reference.
func (g *Git) DiffStat(ref string) (string, error) {
	return g.exec("diff", "--stat", "--no-color", ref)
}

// GetDefaultBranch attempts to determine the default branch of the repository.
func (g *Git) GetDefaultBranch() (string, error) {
	// 1. Try to get the symbolic ref of HEAD (works for non-bare repos and some bare repos)
	out, err := g.exec("symbolic-ref", "--short", "HEAD")
	if err == nil && out != "" {
		return out, nil
	}

	// 2. If that fails (e.g. detached HEAD), try to find the remote HEAD
	out, err = g.exec("symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil && out != "" {
		// Output looks like "origin/main", we want "main"
		return strings.TrimPrefix(out, "origin/"), nil
	}

	// 3. Fallback: list branches and look for main/master
	branches, err := g.ListBranches()
	if err != nil {
		return "", err
	}
	if slices.Contains(branches, "main") {
		return "main", nil
	}
	if slices.Contains(branches, "master") {
		return "master", nil
	}

	if len(branches) > 0 {
		return branches[0], nil
	}

	return "", fmt.Errorf("could not determine default branch")
}
