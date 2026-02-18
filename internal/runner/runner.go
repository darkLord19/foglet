package runner

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/config"
	"github.com/darkLord19/foglet/internal/git"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
	"github.com/darkLord19/foglet/internal/util"
)

// Runner orchestrates AI task execution
type Runner struct {
	repoPath  string
	configDir string
	taskStore *task.Store
	state     *state.Store
	mu        sync.Mutex
	active    map[string]*activeRun
}

// New creates a new runner
func New(repoPath, configDir string) (*Runner, error) {
	// Create task store
	store, err := task.NewStore(configDir)
	if err != nil {
		return nil, err
	}

	return &Runner{
		repoPath:  repoPath,
		configDir: configDir,
		taskStore: store,
		active:    make(map[string]*activeRun),
	}, nil
}

// SetStateStore sets the state persistence backend used for session workflows.
func (r *Runner) SetStateStore(store *state.Store) {
	r.state = store
}

// Execute runs a task
func (r *Runner) Execute(t *task.Task) error {
	return r.executeWithRepoPath(r.repoPath, t)
}

// ExecuteInRepo runs a task against an explicit repository path.
func (r *Runner) ExecuteInRepo(repoPath string, t *task.Task) error {
	return r.executeWithRepoPath(repoPath, t)
}

func (r *Runner) executeWithRepoPath(repoPath string, t *task.Task) error {
	// Save initial state
	if err := r.taskStore.Save(t); err != nil {
		return err
	}

	// Create worktree
	if err := r.createWorktree(repoPath, t); err != nil {
		return r.handleTaskError(t, repoPath, err)
	}

	// Run setup
	if err := r.runSetup(t); err != nil {
		return r.handleTaskError(t, repoPath, err)
	}

	// Run AI
	if err := r.runAI(t); err != nil {
		return r.handleTaskError(t, repoPath, err)
	}

	// Validate
	if t.Options.Validate {
		if err := r.runValidation(t); err != nil {
			return r.handleTaskError(t, repoPath, err)
		}
	}

	// Commit
	if t.Options.Commit {
		if err := r.commitChanges(t); err != nil {
			return r.handleTaskError(t, repoPath, err)
		}
	}

	// Create PR
	if t.Options.CreatePR {
		if err := r.createPR(t); err != nil {
			return r.handleTaskError(t, repoPath, err)
		}
	}

	// Mark complete
	t.TransitionTo(task.StateCompleted)
	r.taskStore.Save(t)

	if r.notificationsEnabled() {
		msg := fmt.Sprintf("Finished successfully on %s (%s)", t.Branch, repoPath)
		util.Notify("Fog Task Complete", msg, "")
	}

	return nil
}

func (r *Runner) handleTaskError(t *task.Task, repoPath string, err error) error {
	t.SetError(err)
	r.taskStore.Save(t)
	if r.notificationsEnabled() {
		msg := fmt.Sprintf("Failed on %s (%s): %v", t.Branch, repoPath, err)
		util.Notify("Fog Task Failed", msg, "")
	}
	return err
}

func (r *Runner) createWorktree(repoPath string, t *task.Task) error {
	t.TransitionTo(task.StateSetup)
	r.taskStore.Save(t)

	worktreePath, err := r.createWorktreePath(repoPath, t.Branch, t.Options.BaseBranch)
	if err != nil {
		return err
	}
	t.WorktreePath = worktreePath

	return nil
}

func (r *Runner) runSetup(t *task.Task) error {
	if t.Options.SetupCmd == "" {
		return nil
	}

	cmd := exec.Command("sh", "-c", t.Options.SetupCmd)
	cmd.Dir = t.WorktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("setup failed: %w\n%s", err, output)
	}

	return nil
}

func (r *Runner) runAI(t *task.Task) error {
	t.TransitionTo(task.StateAIRunning)
	r.taskStore.Save(t)

	// Get AI tool
	tool, err := ai.GetTool(t.AITool)
	if err != nil {
		return err
	}

	if !tool.IsAvailable() {
		return fmt.Errorf("AI tool %s not available", t.AITool)
	}

	// Execute AI
	result, err := tool.Execute(context.Background(), t.WorktreePath, t.Prompt)
	if err != nil {
		if result != nil && result.Output != "" {
			return fmt.Errorf("AI tool failed: %w\nOutput:\n%s", err, result.Output)
		}
		return err
	}

	if !result.Success {
		return fmt.Errorf("AI execution failed: %s", result.Output)
	}

	// Store AI output in metadata
	if t.Metadata == nil {
		t.Metadata = make(map[string]interface{})
	}
	t.Metadata["ai_output"] = result.Output

	return nil
}

func (r *Runner) runValidation(t *task.Task) error {
	if t.Options.ValidateCmd == "" {
		return nil
	}

	t.TransitionTo(task.StateValidating)
	r.taskStore.Save(t)

	cmd := exec.Command("sh", "-c", t.Options.ValidateCmd)
	cmd.Dir = t.WorktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validation failed: %w\n%s", err, output)
	}

	return nil
}

func (r *Runner) commitChanges(t *task.Task) error {
	t.TransitionTo(task.StateCommitted)
	r.taskStore.Save(t)

	// Check if there are changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = t.WorktreePath
	output, _ := cmd.Output()

	if len(output) == 0 {
		// No changes to commit
		return nil
	}

	// Stage all changes
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = t.WorktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Commit with explicit message when provided, otherwise generated message.
	commitMsg := strings.TrimSpace(t.Options.CommitMsg)
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("feat: %s\n\nGenerated by Fog AI task %s", t.Prompt, t.ID)
	}

	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = t.WorktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}

func (r *Runner) createPR(t *task.Task) error {
	t.TransitionTo(task.StatePRCreated)
	r.taskStore.Save(t)

	// Use GitHub CLI
	if !commandExists("gh") {
		return fmt.Errorf("gh CLI not available")
	}

	// Create PR
	title := resolvePRTitle(t.Options.PRTitle, t.Prompt)
	body := fmt.Sprintf("Generated by Fog AI\n\nTask ID: %s\nAI Tool: %s\n\nPrompt:\n%s",
		t.ID, t.AITool, t.Prompt)

	cmd := exec.Command("gh", "pr", "create",
		"--base", t.Options.BaseBranch,
		"--head", t.Branch,
		"--title", title,
		"--body", body)
	cmd.Dir = t.WorktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create PR failed: %w\n%s", err, output)
	}

	// Store PR URL in metadata
	prURL := strings.TrimSpace(string(output))
	if t.Metadata == nil {
		t.Metadata = make(map[string]interface{})
	}
	t.Metadata["pr_url"] = prURL

	return nil
}

func (r *Runner) GetTask(id string) (*task.Task, error) {
	return r.taskStore.Get(id)
}

func (r *Runner) ListTasks() ([]*task.Task, error) {
	return r.taskStore.List()
}

func (r *Runner) ListActiveTasks() ([]*task.Task, error) {
	return r.taskStore.ListActive()
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (r *Runner) createWorktreePath(repoPath, branch, baseBranch string) (string, error) {
	return r.createWorktreePathWithName(repoPath, branch, branch, baseBranch)
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
