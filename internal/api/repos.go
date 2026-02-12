package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	fogenv "github.com/darkLord19/foglet/internal/env"
	foggithub "github.com/darkLord19/foglet/internal/github"
	"github.com/darkLord19/foglet/internal/state"
)

var (
	discoverReposFn    = discoverGitHubReposWithToken
	importReposFn      = importSelectedRepos
	runGitCommandFn    = runGitCommand
	repoSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
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

	token, found, err := s.stateStore.GetGitHubToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "github token not configured", http.StatusBadRequest)
		return
	}

	repos, err := discoverReposFn(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	token, found, err := s.stateStore.GetGitHubToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "github token not configured", http.StatusBadRequest)
		return
	}

	discovered, err := discoverReposFn(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	available := make(map[string]foggithub.Repo, len(discovered))
	for _, repo := range discovered {
		name, err := canonicalRepoName(repo)
		if err != nil {
			continue
		}
		available[name] = repo
	}

	selected := make([]foggithub.Repo, 0, len(req.Repos))
	for _, raw := range req.Repos {
		fullName := strings.TrimSpace(raw)
		if _, _, err := splitRepoFullName(fullName); err != nil {
			http.Error(w, fmt.Sprintf("invalid repo name %q", raw), http.StatusBadRequest)
			return
		}
		repo, ok := available[fullName]
		if !ok {
			http.Error(w, fmt.Sprintf("repo %q is not accessible by configured token", fullName), http.StatusBadRequest)
			return
		}
		selected = append(selected, repo)
	}

	fogHome, err := fogenv.FogHome()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imported, err := importReposFn(fogHome, s.stateStore, token, selected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importReposResponse{
		Imported: imported,
	})
}

func discoverGitHubReposWithToken(token string) ([]foggithub.Repo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := foggithub.NewClient(strings.TrimSpace(token))
	return client.ListRepos(ctx)
}

func importSelectedRepos(fogHome string, store *state.Store, token string, repos []foggithub.Repo) ([]string, error) {
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

		if err := ensureBareRepoInitialized(token, repo, barePath, basePath); err != nil {
			return nil, err
		}

		host := repoHost(repo.CloneURL)
		if _, err := store.UpsertRepo(state.Repo{
			Name:             fullName,
			URL:              repo.CloneURL,
			Host:             host,
			Owner:            owner,
			Repo:             name,
			BarePath:         barePath,
			BaseWorktreePath: basePath,
			DefaultBranch:    repo.DefaultBranch,
		}); err != nil {
			return nil, err
		}
		imported = append(imported, fullName)
	}

	return imported, nil
}

func canonicalRepoName(repo foggithub.Repo) (string, error) {
	fullName := strings.TrimSpace(repo.FullName)
	if fullName == "" {
		owner := strings.TrimSpace(repo.OwnerLogin)
		name := strings.TrimSpace(repo.Name)
		if owner == "" || name == "" {
			return "", fmt.Errorf("repo full name is required")
		}
		fullName = owner + "/" + name
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
	if !repoSegmentPattern.MatchString(owner) || !repoSegmentPattern.MatchString(name) {
		return "", "", fmt.Errorf("repo contains invalid characters")
	}
	return owner, name, nil
}

func ensureBareRepoInitialized(token string, repo foggithub.Repo, barePath, basePath string) error {
	if _, err := os.Stat(barePath); errorsIsNotExist(err) {
		if err := cloneBareRepoWithToken(token, repo.CloneURL, barePath); err != nil {
			return fmt.Errorf("clone bare repository %s: %w", repo.FullName, err)
		}
	} else if err != nil {
		return fmt.Errorf("check bare repo path %s: %w", barePath, err)
	}

	if _, err := os.Stat(basePath); errorsIsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
			return fmt.Errorf("create base worktree parent: %w", err)
		}
		if err := runGitCommandFn("--git-dir", barePath, "worktree", "add", basePath); err != nil {
			return fmt.Errorf("create base worktree for %s: %w", repo.FullName, err)
		}
	} else if err != nil {
		return fmt.Errorf("check base worktree path %s: %w", basePath, err)
	}

	return nil
}

func cloneBareRepoWithToken(token, cloneURL, barePath string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	headers := []string{
		"http.extraHeader=Authorization: Bearer " + token,
		"http.extraHeader=Authorization: Basic " + basicAuthCredential(token),
	}

	var lastErr error
	for _, header := range headers {
		err := runGitCommandFn("-c", header, "clone", "--bare", cloneURL, barePath)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("clone failed")
}

func runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(sanitizeGitArgs(args), " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sanitizeGitArgs(args []string) []string {
	sanitized := make([]string, len(args))
	copy(sanitized, args)

	for i, arg := range sanitized {
		if strings.HasPrefix(arg, "http.extraHeader=Authorization:") {
			sanitized[i] = "http.extraHeader=Authorization: ***"
		}
	}

	return sanitized
}

func basicAuthCredential(token string) string {
	value := "x-access-token:" + strings.TrimSpace(token)
	return base64.StdEncoding.EncodeToString([]byte(value))
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
