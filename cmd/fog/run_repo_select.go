package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	fogenv "github.com/darkLord19/foglet/internal/env"
	foggithub "github.com/darkLord19/foglet/internal/github"
	"github.com/darkLord19/foglet/internal/state"
	"golang.org/x/term"
)

var (
	listGitHubReposFn = listGitHubRepos
	readLineFn        = readLine
	stdinIsTTYFn      = stdinIsTTY

	repoSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func resolveRepoNameForRun(flagRepo string, store *state.Store) (string, error) {
	flagRepo = strings.TrimSpace(flagRepo)
	if flagRepo != "" {
		return flagRepo, nil
	}

	if !stdinIsTTYFn() {
		return "", fmt.Errorf("--repo is required (owner/repo); run 'fog repos discover' to list accessible repos")
	}

	token, found, err := store.GetGitHubToken()
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("github token not configured; run `fog setup` first")
	}

	repos, err := listGitHubReposFn(token)
	if err != nil {
		return "", err
	}
	if len(repos) == 0 {
		return "", fmt.Errorf("no accessible repositories found for configured token")
	}

	fmt.Println("Available repositories:")
	for i, repo := range repos {
		fmt.Printf("  %d. %s\n", i+1, repoAlias(repo))
	}

	input, err := readLineFn("Select repository number: ")
	if err != nil {
		return "", err
	}
	idx, err := parseIndex(input, len(repos))
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(repoAlias(repos[idx]))
	if name == "" {
		return "", errors.New("selected repository has no name")
	}
	return name, nil
}

func ensureRepoRegisteredForRun(repoName string, store *state.Store, fogHome string) (state.Repo, error) {
	repoName = strings.TrimSpace(repoName)
	if repoName == "" {
		return state.Repo{}, fmt.Errorf("repo name is required")
	}

	if repo, found, err := store.GetRepoByName(repoName); err != nil {
		return state.Repo{}, err
	} else if found {
		return repo, nil
	}

	token, found, err := store.GetGitHubToken()
	if err != nil {
		return state.Repo{}, err
	}
	if !found {
		return state.Repo{}, fmt.Errorf("github token not configured; run `fog setup` first")
	}

	repos, err := listGitHubReposFn(token)
	if err != nil {
		return state.Repo{}, err
	}

	match, ok := findRepoByFullName(repos, repoName)
	if !ok {
		return state.Repo{}, fmt.Errorf("repo %q is not accessible by configured token", repoName)
	}

	fullName, owner, name, err := canonicalRepoIdentity(match)
	if err != nil {
		return state.Repo{}, err
	}

	managedReposDir := fogenv.ManagedReposDir(fogHome)
	if err := os.MkdirAll(managedReposDir, 0o755); err != nil {
		return state.Repo{}, fmt.Errorf("create managed repos dir: %w", err)
	}

	repoDir := filepath.Join(managedReposDir, owner, name)
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return state.Repo{}, fmt.Errorf("create repo dir %s: %w", repoDir, err)
	}
	barePath := filepath.Join(repoDir, "repo.git")
	basePath := filepath.Join(repoDir, "base")

	if err := ensureBareRepoInitialized(token, match, barePath, basePath); err != nil {
		return state.Repo{}, err
	}

	host := repoHost(match.CloneURL)
	if _, err := store.UpsertRepo(state.Repo{
		Name:             fullName,
		URL:              match.CloneURL,
		Host:             host,
		Owner:            owner,
		Repo:             name,
		BarePath:         barePath,
		BaseWorktreePath: basePath,
		DefaultBranch:    match.DefaultBranch,
	}); err != nil {
		return state.Repo{}, err
	}

	repo, found, err := store.GetRepoByName(fullName)
	if err != nil {
		return state.Repo{}, err
	}
	if !found {
		return state.Repo{}, fmt.Errorf("managed repo %q disappeared after import", fullName)
	}
	return repo, nil
}

func listGitHubRepos(token string) ([]foggithub.Repo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := foggithub.NewClient(strings.TrimSpace(token))
	return client.ListRepos(ctx)
}

func stdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func canonicalRepoIdentity(repo foggithub.Repo) (fullName, owner, name string, err error) {
	fullName = strings.TrimSpace(repo.FullName)
	owner = strings.TrimSpace(repo.OwnerLogin)
	name = strings.TrimSpace(repo.Name)

	if fullName == "" && owner != "" && name != "" {
		fullName = owner + "/" + name
	}

	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("repo must be in owner/repo format")
	}
	if owner == "" {
		owner = strings.TrimSpace(parts[0])
	}
	if name == "" {
		name = strings.TrimSpace(parts[1])
	}
	if owner == "" || name == "" {
		return "", "", "", fmt.Errorf("repo owner and name are required")
	}
	if !repoSegmentPattern.MatchString(owner) || !repoSegmentPattern.MatchString(name) {
		return "", "", "", fmt.Errorf("repo contains invalid characters")
	}

	fullName = owner + "/" + name
	return fullName, owner, name, nil
}

func findRepoByFullName(repos []foggithub.Repo, fullName string) (foggithub.Repo, bool) {
	fullName = strings.TrimSpace(fullName)
	for _, repo := range repos {
		if strings.TrimSpace(repo.FullName) == fullName {
			return repo, true
		}
	}
	return foggithub.Repo{}, false
}
