package ghcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/darkLord19/foglet/internal/proc"
)

// Repo represents a GitHub repository as returned by gh repo list
type Repo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	NameWithOwner    string `json:"nameWithOwner"`
	URL              string `json:"url"`
	IsPrivate        bool   `json:"isPrivate"`
	DefaultBranchRef struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

var ErrGhNotFound = errors.New("gh CLI not found")

var (
	execCommand = exec.Command
	ghPathFn    = ghPath
	procRun     = proc.Run
)

// IsGhAvailable checks if the gh CLI tool is installed and available in the PATH.
func IsGhAvailable() bool {
	return ghPathFn() != ""
}

// IsGhAuthenticated checks if the user is currently authenticated with GitHub via the gh CLI.
func IsGhAuthenticated() bool {
	// "gh auth status" returns exit code 0 if authenticated, non-zero otherwise.
	gh := ghPathFn()
	if gh == "" {
		return false
	}
	cmd := execCommand(gh, "auth", "status")
	return cmd.Run() == nil
}

// DiscoverRepos fetches the list of repositories available to the authenticated user.
func DiscoverRepos() ([]Repo, error) {
	gh := ghPathFn()
	if gh == "" {
		return nil, ErrGhNotFound
	}

	// --json fields: id,name,nameWithOwner,url,isPrivate,defaultBranchRef,owner
	// --limit 200 to get a reasonable number of repos (per owner)
	const (
		repoLimit = 200
		orgLimit  = 100
		fields    = "id,name,nameWithOwner,url,isPrivate,defaultBranchRef,owner"
	)

	repos, err := listRepos(gh, "", fields, repoLimit)
	if err != nil {
		return nil, err
	}

	orgs, err := listOrgs(gh, orgLimit)
	if err != nil {
		return nil, err
	}

	for _, org := range orgs {
		orgRepos, err := listRepos(gh, org, fields, repoLimit)
		if err != nil {
			return nil, err
		}
		repos = append(repos, orgRepos...)
	}

	repos = dedupeReposByFullName(repos)

	return repos, nil
}

// CloneRepo clones a repository by its full name (owner/repo) to the destination path.
// It uses --bare clone as required by the application architecture.
func CloneRepo(fullName, destPath string) error {
	gh := ghPathFn()
	if gh == "" {
		return ErrGhNotFound
	}

	formatOutput := func(output []byte) string {
		msg := strings.TrimSpace(string(output))
		if len(msg) > 4096 {
			msg = msg[:4096] + "..."
		}
		if msg != "" {
			msg = "\n" + msg
		}
		return msg
	}

	// gh repo clone <repo> <directory> -- <git-args>
	// Prefer a blobless filter for faster imports on large repos.
	args := []string{"repo", "clone", fullName, destPath, "--", "--bare", "--filter=blob:none"}
	cmd := execCommand(gh, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// Some environments ship an older git that doesn't support --filter.
	// Retry without the filter in that case.
	msgLower := strings.ToLower(string(output))
	if strings.Contains(msgLower, "--filter") && strings.Contains(msgLower, "unknown option") {
		if removeErr := os.RemoveAll(destPath); removeErr != nil {
			return fmt.Errorf("cleanup failed after clone retry: %w", removeErr)
		}
		cmd = execCommand(gh, "repo", "clone", fullName, destPath, "--", "--bare")
		output, err = cmd.CombinedOutput()
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("gh repo clone failed: %w%s", err, formatOutput(output))
}

func listOrgs(gh string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 30
	}

	cmd := execCommand(gh, "org", "list", "--limit", strconv.Itoa(limit))
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			msg = "\n" + msg
		}
		return nil, fmt.Errorf("gh org list failed: %w%s", err, msg)
	}

	seen := make(map[string]struct{})
	orgs := make([]string, 0)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		orgs = append(orgs, line)
	}

	return orgs, nil
}

func listRepos(gh string, owner string, fields string, limit int) ([]Repo, error) {
	if fields == "" {
		fields = "id,name,nameWithOwner,url,isPrivate,defaultBranchRef,owner"
	}
	if limit <= 0 {
		limit = 30
	}

	args := []string{"repo", "list"}
	if strings.TrimSpace(owner) != "" {
		args = append(args, owner)
	}
	args = append(args, "--json", fields, "--limit", strconv.Itoa(limit))

	cmd := execCommand(gh, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			msg = "\n" + msg
		}
		if strings.TrimSpace(owner) != "" {
			return nil, fmt.Errorf("gh repo list %s failed: %w%s", owner, err, msg)
		}
		return nil, fmt.Errorf("gh repo list failed: %w%s", err, msg)
	}

	var repos []Repo
	if err := json.Unmarshal(output, &repos); err != nil {
		if strings.TrimSpace(owner) != "" {
			return nil, fmt.Errorf("failed to parse gh repo list %s output: %w", owner, err)
		}
		return nil, fmt.Errorf("failed to parse gh repo list output: %w", err)
	}

	return repos, nil
}

func dedupeReposByFullName(repos []Repo) []Repo {
	seen := make(map[string]struct{}, len(repos))
	out := make([]Repo, 0, len(repos))

	for _, repo := range repos {
		fullName := strings.TrimSpace(repo.NameWithOwner)
		if fullName == "" {
			// Fallback to URL/ID to avoid accidental duplicates when nameWithOwner is missing.
			fullName = strings.TrimSpace(repo.URL)
		}
		if fullName == "" {
			fullName = strings.TrimSpace(repo.ID)
		}
		if fullName == "" {
			continue
		}
		if _, ok := seen[fullName]; ok {
			continue
		}
		seen[fullName] = struct{}{}
		out = append(out, repo)
	}

	return out
}

func ghPath() string {
	if path, err := exec.LookPath("gh"); err == nil && strings.TrimSpace(path) != "" {
		return path
	}

	exeName := "gh"
	if runtime.GOOS == "windows" {
		exeName = "gh.exe"
	}

	for _, dir := range fallbackBinDirs() {
		candidate := filepath.Join(dir, exeName)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS == "windows" {
			return candidate
		}
		if info.Mode()&0o111 != 0 {
			return candidate
		}
	}

	return ""
}

func fallbackBinDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/opt/homebrew/sbin",
		"/usr/local/sbin",
		"/usr/bin",
		"/bin",
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
		)
	}

	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}

// CreatePR creates a pull request for the repository at repoPath.
func CreatePR(repoPath, title, body, base, head string, draft bool) (string, error) {
	gh := ghPathFn()
	if gh == "" {
		return "", ErrGhNotFound
	}

	args := []string{"pr", "create",
		"--base", base,
		"--head", head,
		"--title", title,
		"--body", body,
	}
	if draft {
		args = append(args, "--draft")
	}

	cmd := execCommand(gh, args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if len(msg) > 4096 {
			msg = msg[:4096] + "..."
		}
		if msg != "" {
			msg = "\n" + msg
		}
		return "", fmt.Errorf("gh pr create failed: %w%s", err, msg)
	}

	return strings.TrimSpace(string(output)), nil
}

// CreatePRWithContext creates a pull request for the repository at repoPath and
// allows the operation to be canceled via ctx.
func CreatePRWithContext(ctx context.Context, repoPath, title, body, base, head string, draft bool) (string, error) {
	gh := ghPathFn()
	if gh == "" {
		return "", ErrGhNotFound
	}

	args := []string{"pr", "create",
		"--base", base,
		"--head", head,
		"--title", title,
		"--body", body,
	}
	if draft {
		args = append(args, "--draft")
	}

	output, err := procRun(ctx, repoPath, gh, args...)
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if len(msg) > 4096 {
			msg = msg[:4096] + "..."
		}
		if msg != "" {
			msg = "\n" + msg
		}
		return "", fmt.Errorf("gh pr create failed: %w%s", err, msg)
	}

	return strings.TrimSpace(string(output)), nil
}
