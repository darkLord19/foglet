package ghcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// IsGhAvailable checks if the gh CLI tool is installed and available in the PATH.
func IsGhAvailable() bool {
	return ghPath() != ""
}

// IsGhAuthenticated checks if the user is currently authenticated with GitHub via the gh CLI.
func IsGhAuthenticated() bool {
	// "gh auth status" returns exit code 0 if authenticated, non-zero otherwise.
	gh := ghPath()
	if gh == "" {
		return false
	}
	cmd := exec.Command(gh, "auth", "status")
	return cmd.Run() == nil
}

// DiscoverRepos fetches the list of repositories available to the authenticated user.
func DiscoverRepos() ([]Repo, error) {
	gh := ghPath()
	if gh == "" {
		return nil, ErrGhNotFound
	}

	// --json fields: id,name,nameWithOwner,url,isPrivate,defaultBranchRef,owner
	// --limit 200 to get a reasonable number of recent repos
	cmd := exec.Command(gh, "repo", "list", "--json", "id,name,nameWithOwner,url,isPrivate,defaultBranchRef,owner", "--limit", "200")
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			msg = "\n" + msg
		}
		return nil, fmt.Errorf("gh repo list failed: %w%s", err, msg)
	}

	var repos []Repo
	if err := json.Unmarshal(output, &repos); err != nil {
		return nil, fmt.Errorf("failed to parse gh repo list output: %w", err)
	}

	return repos, nil
}

// CloneRepo clones a repository by its full name (owner/repo) to the destination path.
// It uses --bare clone as required by the application architecture.
func CloneRepo(fullName, destPath string) error {
	gh := ghPath()
	if gh == "" {
		return ErrGhNotFound
	}

	// gh repo clone <repo> <directory> -- <git-args>
	cmd := exec.Command(gh, "repo", "clone", fullName, destPath, "--", "--bare")

	// Capture output for error reporting
	if output, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(output))
		if len(msg) > 4096 {
			msg = msg[:4096] + "..."
		}
		if msg != "" {
			msg = "\n" + msg
		}
		return fmt.Errorf("gh repo clone failed: %w%s", err, msg)
	}

	return nil
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
