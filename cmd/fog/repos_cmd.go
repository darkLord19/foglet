package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	fogenv "github.com/darkLord19/wtx/internal/env"
	foggithub "github.com/darkLord19/wtx/internal/github"
	"github.com/darkLord19/wtx/internal/state"
	"github.com/spf13/cobra"
)

var (
	reposJSONFlag   bool
	reposSelectFlag string
	gitRunner       = runGitCommand
)

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage Fog repositories",
}

var reposDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "List repositories accessible by the configured GitHub PAT",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runReposDiscover(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var reposImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Select and register repositories from GitHub",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runReposImport(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var reposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories already registered in Fog",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runReposList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	reposDiscoverCmd.Flags().BoolVar(&reposJSONFlag, "json", false, "Output JSON")
	reposImportCmd.Flags().StringVar(&reposSelectFlag, "select", "", "Comma-separated GitHub full names to import (e.g. org/repo,org/repo2)")

	reposCmd.AddCommand(reposDiscoverCmd)
	reposCmd.AddCommand(reposImportCmd)
	reposCmd.AddCommand(reposListCmd)
	rootCmd.AddCommand(reposCmd)
}

func runReposDiscover() error {
	repos, err := discoverGitHubRepos()
	if err != nil {
		return err
	}

	if reposJSONFlag {
		data, err := json.MarshalIndent(repos, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(repos) == 0 {
		fmt.Println("No accessible repositories found for configured token")
		return nil
	}

	fmt.Printf("%-30s %-8s %s\n", "FULL NAME", "PRIVATE", "DEFAULT BRANCH")
	fmt.Println(strings.Repeat("-", 70))
	for _, repo := range repos {
		fmt.Printf("%-30s %-8t %s\n", repo.FullName, repo.Private, repo.DefaultBranch)
	}

	return nil
}

func runReposImport() error {
	repos, err := discoverGitHubRepos()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories available to import")
		return nil
	}

	selected, err := selectRepos(repos, reposSelectFlag)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		fmt.Println("No repositories selected")
		return nil
	}

	fogHome, err := fogenv.FogHome()
	if err != nil {
		return err
	}

	store, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	token, found, err := store.GetGitHubToken()
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("github token not configured; run `fog setup` first")
	}

	managedReposDir := fogenv.ManagedReposDir(fogHome)
	if err := os.MkdirAll(managedReposDir, 0o755); err != nil {
		return fmt.Errorf("create managed repos dir: %w", err)
	}

	for _, repo := range selected {
		alias := repoAlias(repo)
		repoDir := filepath.Join(managedReposDir, alias)
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return fmt.Errorf("create repo dir %s: %w", repoDir, err)
		}
		barePath := filepath.Join(repoDir, "repo.git")
		basePath := filepath.Join(repoDir, "base")

		if err := ensureBareRepoInitialized(token, repo, barePath, basePath); err != nil {
			return err
		}

		host := repoHost(repo.CloneURL)
		_, err := store.UpsertRepo(state.Repo{
			Name:             alias,
			URL:              repo.CloneURL,
			Host:             host,
			Owner:            repo.OwnerLogin,
			Repo:             repo.Name,
			BarePath:         barePath,
			BaseWorktreePath: basePath,
			DefaultBranch:    repo.DefaultBranch,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Imported %s as %s\n", repo.FullName, alias)
	}

	return nil
}

func runReposList() error {
	fogHome, err := fogenv.FogHome()
	if err != nil {
		return err
	}
	store, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	repos, err := store.ListRepos()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered")
		return nil
	}

	fmt.Printf("%-20s %-40s %s\n", "NAME", "URL", "DEFAULT BRANCH")
	fmt.Println(strings.Repeat("-", 90))
	for _, repo := range repos {
		fmt.Printf("%-20s %-40s %s\n", repo.Name, repo.URL, repo.DefaultBranch)
	}

	return nil
}

func discoverGitHubRepos() ([]foggithub.Repo, error) {
	fogHome, err := fogenv.FogHome()
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(fogHome)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()

	token, found, err := store.GetGitHubToken()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("github token not configured; run `fog setup` first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := foggithub.NewClient(token)
	repos, err := client.ListRepos(ctx)
	if err != nil {
		return nil, err
	}

	return repos, nil
}

func ensureBareRepoInitialized(token string, repo foggithub.Repo, barePath, basePath string) error {
	if _, err := os.Stat(barePath); errorsIsNotExist(err) {
		header := "http.extraHeader=Authorization: Bearer " + token
		if err := gitRunner(nil, "-c", header, "clone", "--bare", repo.CloneURL, barePath); err != nil {
			return fmt.Errorf("clone bare repository %s: %w", repo.FullName, err)
		}
	} else if err != nil {
		return fmt.Errorf("check bare repo path %s: %w", barePath, err)
	}

	if _, err := os.Stat(basePath); errorsIsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
			return fmt.Errorf("create base worktree parent: %w", err)
		}
		if err := gitRunner(nil, "--git-dir", barePath, "worktree", "add", basePath); err != nil {
			return fmt.Errorf("create base worktree for %s: %w", repo.FullName, err)
		}
	} else if err != nil {
		return fmt.Errorf("check base worktree path %s: %w", basePath, err)
	}

	return nil
}

func runGitCommand(extraEnv []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func selectRepos(repos []foggithub.Repo, selectFlag string) ([]foggithub.Repo, error) {
	selectFlag = strings.TrimSpace(selectFlag)
	if selectFlag == "" {
		fmt.Println("Available repositories:")
		for i, repo := range repos {
			fmt.Printf("  %d. %s\n", i+1, repo.FullName)
		}
		input, err := readLine("Select repository numbers (comma-separated): ")
		if err != nil {
			return nil, err
		}
		indexes, err := parseIndexes(input, len(repos))
		if err != nil {
			return nil, err
		}
		selected := make([]foggithub.Repo, 0, len(indexes))
		for _, idx := range indexes {
			selected = append(selected, repos[idx])
		}
		return selected, nil
	}

	want := make(map[string]struct{})
	for _, name := range strings.Split(selectFlag, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			want[name] = struct{}{}
		}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("--select cannot be empty")
	}

	selected := make([]foggithub.Repo, 0, len(want))
	for _, repo := range repos {
		if _, ok := want[repo.FullName]; ok {
			selected = append(selected, repo)
			delete(want, repo.FullName)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for name := range want {
			missing = append(missing, name)
		}
		return nil, fmt.Errorf("unknown repositories in --select: %s", strings.Join(missing, ", "))
	}

	return selected, nil
}

func parseIndex(input string, max int) (int, error) {
	i, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, fmt.Errorf("invalid selection %q", input)
	}
	if i < 1 || i > max {
		return 0, fmt.Errorf("selection out of range: %d", i)
	}
	return i - 1, nil
}

func parseIndexes(input string, max int) ([]int, error) {
	parts := strings.Split(input, ",")
	result := make([]int, 0, len(parts))
	seen := make(map[int]struct{})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx, err := parseIndex(part, max)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		result = append(result, idx)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid selections provided")
	}
	return result, nil
}

func repoAlias(repo foggithub.Repo) string {
	full := strings.TrimSpace(repo.FullName)
	if full != "" {
		return strings.ReplaceAll(full, "/", "-")
	}
	if repo.OwnerLogin == "" {
		return repo.Name
	}
	return repo.OwnerLogin + "-" + repo.Name
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
