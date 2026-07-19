package runner

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/git"
)

var nonBranchSlugChar = regexp.MustCompile(`[^a-z0-9]+`)

// ResolveBranch resolves a unique branch name for a session.
// If requested is non-empty, it validates and returns it.
// Otherwise, it generates a slug from the prompt and ensures uniqueness.
func (r *Runner) ResolveBranch(repoPath, requested, prompt string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return validateBranchName(requested)
	}

	prefix := "fog"
	if r.state != nil {
		if stored, found, err := r.state.GetSetting("branch_prefix"); err == nil && found {
			stored = strings.TrimSpace(stored)
			if stored != "" {
				prefix = stored
			}
		}
	}

	slug := slugifyPrompt(prompt)
	baseBranch := strings.Trim(prefix, "/") + "/" + slug
	if len(baseBranch) > 255 {
		baseBranch = strings.Trim(baseBranch[:255], "/.-")
	}

	g := git.New(repoPath)

	truncateWithSuffix := func(base, suffix string) string {
		maxBaseLen := max(255-len(suffix), 1)
		if len(base) > maxBaseLen {
			base = strings.Trim(base[:maxBaseLen], "/.-")
		}
		return base + suffix
	}

	for i := 0; i <= 100; i++ {
		branch := baseBranch
		if i > 0 {
			branch = truncateWithSuffix(baseBranch, fmt.Sprintf("-%d", i))
		}
		if !g.BranchExists(branch) {
			return validateBranchName(branch)
		}
	}

	// Fallback to a short random-ish suffix to avoid exceeding git's 255-byte limit.
	suffix := "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	return validateBranchName(truncateWithSuffix(baseBranch, suffix))
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

func validateBranchName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("branch name cannot be empty")
	}
	if len(value) > 255 {
		return "", fmt.Errorf("branch name exceeds 255 characters")
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return "", fmt.Errorf("branch name cannot start or end with '/'")
	}
	if strings.Contains(value, "..") || strings.Contains(value, "//") || strings.Contains(value, "@{") {
		return "", fmt.Errorf("branch name contains invalid sequence")
	}
	if strings.ContainsAny(value, " ~^:?*[\\") {
		return "", fmt.Errorf("branch name contains invalid character")
	}
	return value, nil
}

func slugifyPrompt(prompt string) string {
	slug := strings.ToLower(strings.TrimSpace(prompt))
	slug = nonBranchSlugChar.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "task-" + time.Now().UTC().Format("20060102150405")
	}
	return slug
}
