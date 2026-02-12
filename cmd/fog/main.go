package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/darkLord19/wtx/internal/env"
	"github.com/darkLord19/wtx/internal/runner"
	"github.com/darkLord19/wtx/internal/state"
	"github.com/darkLord19/wtx/internal/task"
	"github.com/darkLord19/wtx/internal/toolcfg"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagBranch      string
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
	Short: "List all tasks",
	Run: func(cmd *cobra.Command, args []string) {
		if err := listTasks(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <task-id>",
	Short: "Show task status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := showStatus(args[0]); err != nil {
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
	runCmd.Flags().StringVar(&flagTool, "tool", "", "AI tool to use (cursor, claude, aider)")
	runCmd.Flags().StringVar(&flagPrompt, "prompt", "", "Task prompt (required)")
	runCmd.Flags().BoolVar(&flagCommit, "commit", false, "Commit changes after AI completes")
	runCmd.Flags().BoolVar(&flagPR, "pr", false, "Create pull request")
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
	// Get current directory (repo root)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Get config dir
	fogHome, err := env.FogHome()
	if err != nil {
		return err
	}
	configDir := fogHome

	stateStore, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = stateStore.Close() }()

	resolvedTool, err := toolcfg.ResolveTool(flagTool, stateStore, "cli")
	if err != nil {
		return err
	}

	// Create runner
	r, err := runner.New(cwd, configDir)
	if err != nil {
		return err
	}

	// Create task
	t := &task.Task{
		ID:        uuid.New().String(),
		State:     task.StateCreated,
		Branch:    flagBranch,
		Prompt:    flagPrompt,
		AITool:    resolvedTool,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Options: task.Options{
			Commit:      flagCommit,
			CreatePR:    flagPR,
			Validate:    flagValidate,
			BaseBranch:  flagBaseBranch,
			SetupCmd:    flagSetupCmd,
			ValidateCmd: flagValidateCmd,
			Async:       flagAsync,
		},
	}

	fmt.Printf("Starting task %s\n", t.ID)
	fmt.Printf("Branch: %s\n", t.Branch)
	fmt.Printf("AI Tool: %s\n", t.AITool)
	fmt.Printf("Prompt: %s\n", t.Prompt)
	fmt.Println()

	// Execute
	if err := r.Execute(t); err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Task completed in %v\n", t.Duration())
	fmt.Printf("State: %s\n", t.State)
	fmt.Printf("Worktree: %s\n", t.WorktreePath)

	if prURL, ok := t.Metadata["pr_url"].(string); ok {
		fmt.Printf("PR: %s\n", prURL)
	}

	return nil
}

func listTasks() error {
	fogHome, err := env.FogHome()
	if err != nil {
		return err
	}
	configDir := fogHome

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	r, err := runner.New(cwd, configDir)
	if err != nil {
		return err
	}

	tasks, err := r.ListTasks()
	if err != nil {
		return err
	}

	if flagJSON {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return nil
	}

	fmt.Printf("%-36s %-15s %-20s %s\n", "ID", "STATE", "BRANCH", "CREATED")
	fmt.Println(string(make([]byte, 100)))

	for _, t := range tasks {
		fmt.Printf("%-36s %-15s %-20s %s\n",
			t.ID,
			t.State,
			t.Branch,
			t.CreatedAt.Format("2006-01-02 15:04"))
	}

	return nil
}

func showStatus(id string) error {
	fogHome, err := env.FogHome()
	if err != nil {
		return err
	}
	configDir := fogHome

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	r, err := runner.New(cwd, configDir)
	if err != nil {
		return err
	}

	t, err := r.GetTask(id)
	if err != nil {
		return err
	}

	fmt.Printf("Task: %s\n", t.ID)
	fmt.Printf("State: %s\n", t.State)
	fmt.Printf("Branch: %s\n", t.Branch)
	fmt.Printf("AI Tool: %s\n", t.AITool)
	fmt.Printf("Prompt: %s\n", t.Prompt)
	fmt.Printf("Created: %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Duration: %v\n", t.Duration())

	if t.WorktreePath != "" {
		fmt.Printf("Worktree: %s\n", t.WorktreePath)
	}

	if t.Error != "" {
		fmt.Printf("Error: %s\n", t.Error)
	}

	if prURL, ok := t.Metadata["pr_url"].(string); ok {
		fmt.Printf("PR: %s\n", prURL)
	}

	return nil
}
