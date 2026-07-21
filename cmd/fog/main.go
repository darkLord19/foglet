package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/toolcfg"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagBranch      string
	flagRepo        string
	flagTool        string
	flagPrompt      string
	flagCommit      bool
	flagPR          bool
	flagValidate    bool
	flagBaseBranch  string
	flagSetupCmd    string
	flagValidateCmd string
	flagAsync       bool
	flagJSON        bool
	flagPRTitle     string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "fog",
	Short: "Turn your local machine into cloud agents",
	Long:  `Fog orchestrates AI coding tasks using existing AI tools in isolated worktrees`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run an AI coding task",
	Long: `Execute an AI task in an isolated worktree
	
Example:
  fog run \
    --repo owner/repo \
    --branch feature-otp \
    --tool claude \
    --prompt "Add OTP login using Redis" \
    --commit \
    --pr`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTask(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Run: func(cmd *cobra.Command, args []string) {
		if err := listSessions(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <session-id>",
	Short: "Show session status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := showSessionStatus(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("fog version %s\n", version)
	},
}

func init() {
	// run command flags
	runCmd.Flags().StringVar(&flagBranch, "branch", "", "Branch name (required)")
	runCmd.Flags().StringVar(&flagRepo, "repo", "", "Target repository (owner/repo; imported automatically when missing)")
	runCmd.Flags().StringVar(&flagTool, "tool", "", "AI tool to use (cursor, claude, antigravity)")
	runCmd.Flags().StringVar(&flagPrompt, "prompt", "", "Task prompt (required)")
	runCmd.Flags().BoolVar(&flagCommit, "commit", false, "Commit changes after AI completes")
	runCmd.Flags().BoolVar(&flagPR, "pr", false, "Create pull request")
	runCmd.Flags().StringVar(&flagPRTitle, "pr-title", "", "Pull request title (requires --pr)")
	runCmd.Flags().BoolVar(&flagValidate, "validate", false, "Run validation after AI")
	runCmd.Flags().StringVar(&flagBaseBranch, "base", "main", "Base branch for PR")
	runCmd.Flags().StringVar(&flagSetupCmd, "setup-cmd", "", "Setup command to run")
	runCmd.Flags().StringVar(&flagValidateCmd, "validate-cmd", "", "Validation command to run")
	runCmd.Flags().BoolVar(&flagAsync, "async", false, "Run asynchronously")

	runCmd.MarkFlagRequired("branch")
	runCmd.MarkFlagRequired("prompt")

	// list command flags
	listCmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

func runTask() error {
	fogHome, err := env.FogHome()
	if err != nil {
		return err
	}

	stateStore, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = stateStore.Close() }()

	resolvedTool, err := toolcfg.ResolveTool(flagTool, stateStore, "cli")
	if err != nil {
		return err
	}

	repoName, err := resolveRepoNameForRun(flagRepo, stateStore)
	if err != nil {
		return err
	}

	repo, err := ensureRepoRegisteredForRun(repoName, stateStore, fogHome)
	if err != nil {
		return err
	}
	if strings.TrimSpace(repo.BaseWorktreePath) == "" {
		return fmt.Errorf("managed repo %q has no base worktree path", repo.Name)
	}

	// Create runner
	r := runner.New(stateStore)

	baseBranch := strings.TrimSpace(flagBaseBranch)
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(repo.DefaultBranch)
	}
	if baseBranch == "" {
		baseBranch = "main"
	}

	opts := runner.StartSessionOptions{
		RepoName:    repo.Name,
		RepoPath:    repo.BaseWorktreePath,
		Branch:      flagBranch,
		Tool:        resolvedTool,
		Model:       "",
		Prompt:      flagPrompt,
		AutoPR:      flagPR,
		SetupCmd:    flagSetupCmd,
		Validate:    flagValidate,
		ValidateCmd: flagValidateCmd,
		BaseBranch:  baseBranch,
		CommitMsg:   "",
		PRTitle:     flagPRTitle,
	}

	fmt.Printf("Starting session\n")
	fmt.Printf("Branch: %s\n", opts.Branch)
	fmt.Printf("AI Tool: %s\n", opts.Tool)
	fmt.Printf("Prompt: %s\n", opts.Prompt)
	fmt.Println()

	session, run, err := r.StartSession(opts)
	if err != nil {
		return fmt.Errorf("session execution failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Session completed\n")
	fmt.Printf("Run state: %s\n", run.State)
	fmt.Printf("Branch: %s\n", session.Branch)
	fmt.Printf("Worktree: %s\n", session.WorktreePath)

	if session.PRURL != "" {
		fmt.Printf("PR: %s\n", session.PRURL)
	}

	return nil
}

func listSessions() error {
	fogHome, err := env.FogHome()
	if err != nil {
		return err
	}

	stateStore, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = stateStore.Close() }()

	r := runner.New(stateStore)

	sessions, err := r.ListSessions()
	if err != nil {
		return err
	}

	if flagJSON {
		data, err := json.MarshalIndent(sessions, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	fmt.Printf("%-36s %-15s %-20s %s\n", "ID", "STATUS", "BRANCH", "CREATED")
	fmt.Println(string(make([]byte, 100)))

	for _, s := range sessions {
		fmt.Printf("%-36s %-15s %-20s %s\n",
			s.ID,
			s.Status,
			s.Branch,
			s.CreatedAt.Format("2006-01-02 15:04"))
	}

	return nil
}

func showSessionStatus(id string) error {
	fogHome, err := env.FogHome()
	if err != nil {
		return err
	}

	stateStore, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = stateStore.Close() }()

	r := runner.New(stateStore)

	session, found, err := r.GetSession(id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("session not found: %s", id)
	}

	runs, err := r.ListSessionRuns(id)
	if err != nil {
		return err
	}

	fmt.Printf("Session: %s\n", session.ID)
	fmt.Printf("Status: %s\n", session.Status)
	fmt.Printf("Repo: %s\n", session.RepoName)
	fmt.Printf("Branch: %s\n", session.Branch)
	fmt.Printf("AI Tool: %s\n", session.Tool)
	fmt.Printf("AutoPR: %v\n", session.AutoPR)
	fmt.Printf("Created: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))

	if session.WorktreePath != "" {
		fmt.Printf("Worktree: %s\n", session.WorktreePath)
	}
	if session.PRURL != "" {
		fmt.Printf("PR: %s\n", session.PRURL)
	}

	if len(runs) > 0 {
		fmt.Println()
		fmt.Printf("Runs (%d):\n", len(runs))
		for _, run := range runs {
			fmt.Printf("  - %s: %s (%s)\n", run.ID[:8], run.State, run.Prompt)
			if run.Error != "" {
				fmt.Printf("    Error: %s\n", run.Error)
			}
		}
	}

	return nil
}
