package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	fogenv "github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/state"
)

var (
	discoverReposFn     = discoverGitHubRepos
	importReposFn       = importSelectedRepos
	runGitCommandFn     = runGitCommand
	isGhAvailableFn     = ghcli.IsGhAvailable
	isGhAuthenticatedFn = ghcli.IsGhAuthenticated
	repoSegmentPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

type importReposRequest struct {
	Repos []string `json:"repos"`
}

type importReposResponse struct {
	Imported []string `json:"imported"`
}

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repos, err := s.stateStore.ListRepos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(repos)
}

func (s *Server) handleDiscoverRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// No token required, relies on gh CLI authentication
	if !isGhAvailableFn() {
		http.Error(w, "gh CLI is not installed", http.StatusServiceUnavailable)
		return
	}
	if !isGhAuthenticatedFn() {
		http.Error(w, "gh CLI is not authenticated", http.StatusUnauthorized)
		return
	}

	repos, err := discoverReposFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(repos)
}

func (s *Server) handleImportRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req importReposRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Repos) == 0 {
		http.Error(w, "repos is required", http.StatusBadRequest)
		return
	}

	// No token check needed, relies on gh CLI auth which is checked in discover/import steps implicitly or explicitly
	if !isGhAvailableFn() {
		http.Error(w, "gh CLI is not installed", http.StatusServiceUnavailable)
		return
	}
	if !isGhAuthenticatedFn() {
		http.Error(w, "gh CLI is not authenticated", http.StatusUnauthorized)
		return
	}

	discovered, err := discoverReposFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	available := make(map[string]ghcli.Repo, len(discovered))
	for _, repo := range discovered {
		name, err := canonicalRepoName(repo)
		if err != nil {
			continue
		}
		available[name] = repo
	}

	selected := make([]ghcli.Repo, 0, len(req.Repos))
	for _, raw := range req.Repos {
		fullName := strings.TrimSpace(raw)
		if _, _, err := splitRepoFullName(fullName); err != nil {
			http.Error(w, fmt.Sprintf("invalid repo name %q", raw), http.StatusBadRequest)
			return
		}
		repo, ok := available[fullName]
		if !ok {
			http.Error(w, fmt.Sprintf("repo %q is not accessible via gh CLI", fullName), http.StatusBadRequest)
			return
		}
		selected = append(selected, repo)
	}

	fogHome, err := fogenv.FogHome()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imported, err := importReposFn(fogHome, s.stateStore, selected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importReposResponse{
		Imported: imported,
	})
}

func discoverGitHubRepos() ([]ghcli.Repo, error) {
	return ghcli.DiscoverRepos()
}

func importSelectedRepos(fogHome string, store *state.Store, repos []ghcli.Repo) ([]string, error) {
	managedReposDir := fogenv.ManagedReposDir(fogHome)
	if err := os.MkdirAll(managedReposDir, 0o755); err != nil {
		return nil, fmt.Errorf("create managed repos dir: %w", err)
	}

	imported := make([]string, 0, len(repos))
	for _, repo := range repos {
		fullName, err := canonicalRepoName(repo)
		if err != nil {
			return nil, err
		}
		owner, name, err := splitRepoFullName(fullName)
		if err != nil {
			return nil, err
		}

		repoDir := filepath.Join(managedReposDir, owner, name)
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return nil, fmt.Errorf("create repo dir %s: %w", repoDir, err)
		}
		barePath := filepath.Join(repoDir, "repo.git")
		basePath := filepath.Join(repoDir, "base")

		if err := ensureBareRepoInitialized(repo, barePath, basePath); err != nil {
			return nil, err
		}

		host := repoHost(repo.URL)
		if _, err := store.UpsertRepo(state.Repo{
			Name:             fullName,
			URL:              repo.URL,
			Host:             host,
			Owner:            owner,
			Repo:             name,
			BarePath:         barePath,
			BaseWorktreePath: basePath,
			DefaultBranch:    repo.DefaultBranchRef.Name,
		}); err != nil {
			return nil, err
		}
		imported = append(imported, fullName)
	}

	return imported, nil
}

func canonicalRepoName(repo ghcli.Repo) (string, error) {
	fullName := strings.TrimSpace(repo.NameWithOwner)
	if fullName == "" {
		owner := ""
		if repo.Owner.Login != "" {
			owner = strings.TrimSpace(repo.Owner.Login)
		} else {
			// fallback attempt to parse from NameWithOwner if owner struct missing/empty
		}

		name := strings.TrimSpace(repo.Name)
		if owner != "" && name != "" {
			fullName = owner + "/" + name
		}

		if fullName == "" {
			return "", fmt.Errorf("repo full name is required")
		}
	}
	_, _, err := splitRepoFullName(fullName)
	if err != nil {
		return "", err
	}
	return fullName, nil
}

func splitRepoFullName(fullName string) (owner string, name string, err error) {
	fullName = strings.TrimSpace(fullName)
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo must be in owner/repo format")
	}
	owner = strings.TrimSpace(parts[0])
	name = strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("repo owner and name are required")
	}
	if owner == "." || owner == ".." || name == "." || name == ".." {
		return "", "", fmt.Errorf("repo contains invalid segment")
	}
	if !repoSegmentPattern.MatchString(owner) || !repoSegmentPattern.MatchString(name) {
		return "", "", fmt.Errorf("repo contains invalid characters")
	}
	return owner, name, nil
}

func ensureBareRepoInitialized(repo ghcli.Repo, barePath, basePath string) error {
	if _, err := os.Stat(barePath); errorsIsNotExist(err) {
		// Use gh repo clone via ghcli package
		// We pass FullName (owner/repo)
		if err := ghcli.CloneRepo(repo.NameWithOwner, barePath); err != nil {
			return fmt.Errorf("clone bare repository %s: %w", repo.NameWithOwner, err)
		}
	} else if err != nil {
		return fmt.Errorf("check bare repo path %s: %w", barePath, err)
	}

	if _, err := os.Stat(basePath); errorsIsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
			return fmt.Errorf("create base worktree parent: %w", err)
		}
		if err := runGitCommandFn("--git-dir", barePath, "worktree", "add", basePath); err != nil {
			return fmt.Errorf("create base worktree for %s: %w", repo.NameWithOwner, err)
		}
	} else if err != nil {
		return fmt.Errorf("check base worktree path %s: %w", basePath, err)
	}

	return nil
}

func runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func repoHost(cloneURL string) string {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return "github.com"
	}
	if u.Host == "" {
		return "github.com"
	}
	return u.Host
}
