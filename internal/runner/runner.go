package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/config"
	"github.com/darkLord19/foglet/internal/git"
	"github.com/darkLord19/foglet/internal/power"
	"github.com/darkLord19/foglet/internal/state"
)

// Runner orchestrates AI task execution
type Runner struct {
	state    *state.Store
	runs     RunStore
	settings SettingsReader
	tools    ToolFactory
	baseCtx  context.Context
	power    *power.Inhibitor
	mu       sync.Mutex
	active   map[string]*activeRun
}

// New creates a new runner. The state store st is optional (may be nil).
//
// Every path that needs a repository takes it as an argument, so the runner holds
// no repository of its own.
func New(st *state.Store) *Runner {
	r := &Runner{
		state:   st,
		tools:   ai.GetTool,
		baseCtx: context.Background(),
		power:   power.New(),
		active:  make(map[string]*activeRun),
	}
	// Assigned only when non-nil: a nil *state.Store stored in an interface is
	// itself non-nil, which would turn every nil-store guard into a panic.
	if st != nil {
		r.runs = st
		r.settings = st
	}
	return r
}

// SetBaseContext replaces the default context.Background() used as the parent
// for run Contexts. The daemon passes its daemonCtx so that in-flight runs are
// cancelled on shutdown. CLI synchronous calls leave the default (Background()).
func (r *Runner) SetBaseContext(ctx context.Context) {
	r.baseCtx = ctx
}

func (r *Runner) keepAwakeEnabled() bool {
	if r == nil || r.settings == nil {
		return false
	}
	val, found, err := r.settings.GetSetting("keep_awake")
	if err != nil || !found {
		return false
	}
	return val == "true"
}

func (r *Runner) createWorktreePathWithName(repoPath, name, branch, baseBranch string) (string, error) {
	name = strings.TrimSpace(name)
	branch = strings.TrimSpace(branch)
	baseBranch = strings.TrimSpace(baseBranch)

	g := git.New(repoPath)
	if !g.IsRepo() {
		return "", fmt.Errorf("not a git repository: %s", repoPath)
	}
	if name == "" {
		return "", fmt.Errorf("worktree name is required")
	}
	if branch == "" {
		return "", fmt.Errorf("worktree branch is required")
	}

	// Load wtx config to get worktree directory preference
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load wtx config: %w", err)
	}

	root, err := g.GetRepoRoot()
	if err != nil {
		return "", fmt.Errorf("get repo root: %w", err)
	}

	// Construct worktree path using wtx config.
	// WorktreeDir is typically relative to the repo root (default: ../worktrees).
	// If WorktreeDir is absolute, filepath.Join will use it as-is.
	worktreePath := filepath.Join(root, cfg.WorktreeDir, name)

	if g.BranchExists(branch) {
		if err := g.AddWorktree(worktreePath, branch); err != nil {
			return "", fmt.Errorf("create worktree: %w", err)
		}
	} else {
		if baseBranch == "" {
			return "", fmt.Errorf("base branch is required")
		}
		// New branch creation always requires an explicit base
		if err := g.AddWorktreeNewBranch(worktreePath, branch, baseBranch); err != nil {
			return "", fmt.Errorf("create worktree with new branch (start=%s): %w", baseBranch, err)
		}
	}

	return worktreePath, nil
}
