package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/darkLord19/wtx/internal/config"
	"github.com/darkLord19/wtx/internal/editor"
	"github.com/darkLord19/wtx/internal/git"
	"github.com/darkLord19/wtx/internal/metadata"
	"github.com/darkLord19/wtx/internal/tui"
	"github.com/darkLord19/wtx/internal/util"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagJSON    bool
	flagAddJSON bool
	flagEditor  string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "wtx",
	Short: "Git worktree manager",
	Long:  `wtx - Fast, keyboard-driven workspace switcher for Git worktrees`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get current directory
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Launch TUI
		if err := tui.Run(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("wtx version %s\n", version)
	},
}

func init() {
	listCmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")
	addCmd.Flags().BoolVar(&flagAddJSON, "json", false, "Output result as JSON")

	rootCmd.PersistentFlags().StringVar(&flagEditor, "editor", "", "Editor to use (vscode, cursor, neovim, etc)")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

var addCmd = &cobra.Command{
	Use:   "add <name> [branch]",
	Short: "Create a new worktree",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		branch := name
		if len(args) > 1 {
			branch = args[1]
		}

		if err := runAdd(name, branch); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var openCmd = &cobra.Command{
	Use:   "open <name>",
	Short: "Open a worktree in editor",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runOpen(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a worktree",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRemove(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show detailed status of a worktree",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or edit configuration",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runList() error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	// Initialize git
	g := git.New(cwd)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Get worktrees
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	if flagJSON {
		// Output as JSON
		data, err := json.MarshalIndent(worktrees, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	if len(worktrees) == 0 {
		fmt.Println("No worktrees found")
		return nil
	}

	for _, wt := range worktrees {
		fmt.Printf("%-20s %s\n", wt.Name, wt.Path)
		if wt.Branch != "" {
			fmt.Printf("  └─ %s\n", wt.Branch)
		}
	}

	return nil
}

func runAdd(name, branch string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	g := git.New(cwd)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Get repo root
	root, err := g.GetRepoRoot()
	if err != nil {
		return err
	}

	// Determine worktree path
	wtPath := fmt.Sprintf("%s/%s/%s", root, cfg.WorktreeDir, name)

	// Create worktree
	if !flagAddJSON {
		fmt.Printf("Creating worktree '%s' at %s...\n", name, wtPath)
	}
	if g.BranchExists(branch) {
		if err := g.AddWorktree(wtPath, branch); err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
	} else {
		startPoint := cfg.DefaultBranch
		if startPoint == "" {
			startPoint = "HEAD"
		}
		if err := g.AddWorktreeNewBranch(wtPath, branch, startPoint); err != nil {
			return fmt.Errorf("create worktree with new branch: %w", err)
		}
	}

	// Store metadata
	store, err := metadata.New(root)
	if err != nil {
		return err
	}

	wtMeta := &metadata.WorktreeMetadata{
		Path:      wtPath,
		CreatedAt: time.Now(),
	}

	if err := store.SetWorktree(name, wtMeta); err != nil {
		return err
	}

	if !flagAddJSON {
		fmt.Printf("✓ Worktree '%s' created\n", name)
	}

	// Run setup command if configured
	if cfg.SetupCmd != "" {
		if !flagAddJSON {
			fmt.Printf("Running setup command: %s\n", cfg.SetupCmd)
		}
		result := util.RunCommand(cfg.SetupCmd, wtPath)

		// Update metadata with setup results
		wtMeta.SetupRan = true
		wtMeta.SetupOutput = result.Output

		if result.Error != nil {
			if !flagAddJSON {
				fmt.Printf("⚠ Setup command failed (exit %d)\n", result.ExitCode)
				fmt.Printf("Output:\n%s\n", result.Output)
			}
			store.SetWorktree(name, wtMeta)
			return fmt.Errorf("setup command failed: %w", result.Error)
		}

		store.SetWorktree(name, wtMeta)
		if !flagAddJSON {
			fmt.Printf("✓ Setup complete (%v)\n", result.Duration)
		}
	}

	if flagAddJSON {
		result := map[string]interface{}{
			"name":   name,
			"branch": branch,
			"path":   wtPath,
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal add result: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Auto-open if configured
	if err := runOpen(name); err != nil {
		fmt.Printf("Note: Could not auto-open: %v\n", err)
	}

	return nil
}

func runOpen(name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	g := git.New(cwd)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Get worktrees
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return err
	}

	// Find the worktree
	var targetPath string
	for _, wt := range worktrees {
		if wt.Name == name {
			targetPath = wt.Path
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Detect editor
	editorName := flagEditor
	if editorName == "" {
		editorName = cfg.Editor
	}

	ed, err := editor.Detect(editorName)
	if err != nil {
		return fmt.Errorf("no editor available: %w", err)
	}

	fmt.Printf("Opening '%s' in %s...\n", name, ed.Name())

	if err := ed.Open(targetPath, cfg.ReuseWindow); err != nil {
		return fmt.Errorf("open editor: %w", err)
	}

	// Update last opened timestamp
	root, _ := g.GetRepoRoot()
	if root != "" {
		store, _ := metadata.New(root)
		if store != nil {
			store.UpdateLastOpened(name)
		}
	}

	return nil
}

func runRemove(name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	g := git.New(cwd)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Get worktrees
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return err
	}

	// Find the worktree
	var targetPath string
	for _, wt := range worktrees {
		if wt.Name == name {
			targetPath = wt.Path
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Check if it has uncommitted changes
	hasChanges, err := g.HasUncommittedChanges(targetPath)
	if err == nil && hasChanges {
		fmt.Printf("⚠ Worktree '%s' has uncommitted changes\n", name)
		fmt.Println("Options:")
		fmt.Println("  [c] Cancel")
		fmt.Println("  [f] Force delete")

		var choice string
		fmt.Print("\nSelect: ")
		fmt.Scanln(&choice)

		if choice != "f" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Remove worktree
	fmt.Printf("Removing worktree '%s'...\n", name)
	force := hasChanges
	if err := g.RemoveWorktree(targetPath, force); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}

	// Delete metadata
	root, _ := g.GetRepoRoot()
	if root != "" {
		store, _ := metadata.New(root)
		if store != nil {
			store.DeleteWorktree(name)
		}
	}

	fmt.Printf("✓ Worktree '%s' removed\n", name)

	return nil
}

func runStatus(name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	g := git.New(cwd)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Get worktrees
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return err
	}

	// Find the worktree
	var wt *git.Worktree
	for i := range worktrees {
		if worktrees[i].Name == name {
			wt = &worktrees[i]
			break
		}
	}

	if wt == nil {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Get status
	status, err := g.GetStatus(wt.Path)
	if err != nil {
		return err
	}

	// Get metadata
	root, _ := g.GetRepoRoot()
	var wtMeta *metadata.WorktreeMetadata
	if root != "" {
		store, _ := metadata.New(root)
		if store != nil {
			wtMeta, _ = store.GetWorktree(name)
		}
	}

	// Display status
	fmt.Printf("Worktree: %s\n", name)
	fmt.Printf("Path: %s\n", wt.Path)
	fmt.Printf("Branch: %s\n", wt.Branch)
	fmt.Printf("Head: %s\n", wt.Head[:8])

	if status != nil {
		if status.Dirty {
			fmt.Printf("Status: ✗ dirty\n")
		} else {
			fmt.Printf("Status: ● clean\n")
		}

		if status.Ahead > 0 {
			fmt.Printf("Ahead: ↑ %d commits\n", status.Ahead)
		}
		if status.Behind > 0 {
			fmt.Printf("Behind: ↓ %d commits\n", status.Behind)
		}
		if status.Stash {
			fmt.Printf("Stash: Yes\n")
		}
	}

	if wtMeta != nil {
		fmt.Printf("\nMetadata:\n")
		fmt.Printf("  Created: %s\n", wtMeta.CreatedAt.Format("2006-01-02 15:04:05"))
		if !wtMeta.LastOpened.IsZero() {
			fmt.Printf("  Last opened: %s\n", wtMeta.LastOpened.Format("2006-01-02 15:04:05"))
		}
		if wtMeta.SetupRan {
			fmt.Printf("  Setup: ✓ complete\n")
		}
		if !wtMeta.LastValidate.IsZero() {
			if wtMeta.ValidatePass {
				fmt.Printf("  Last validation: ✓ passed (%s)\n", wtMeta.LastValidate.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("  Last validation: ✗ failed (%s)\n", wtMeta.LastValidate.Format("2006-01-02 15:04:05"))
			}
		}
		if wtMeta.DevCommand != "" {
			fmt.Printf("  Dev command: %s\n", wtMeta.DevCommand)
		}
		if len(wtMeta.Ports) > 0 {
			fmt.Printf("  Ports: %v\n", wtMeta.Ports)
		}
	}

	return nil
}

func runConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Display current config
	fmt.Println("Current configuration:")
	fmt.Println()
	fmt.Printf("editor: %s\n", cfg.Editor)
	fmt.Printf("reuse_window: %v\n", cfg.ReuseWindow)
	fmt.Printf("worktree_dir: %s\n", cfg.WorktreeDir)
	fmt.Printf("auto_start_dev: %v\n", cfg.AutoStartDev)
	fmt.Printf("default_branch: %s\n", cfg.DefaultBranch)
	fmt.Printf("setup_cmd: %s\n", cfg.SetupCmd)
	fmt.Printf("validate_cmd: %s\n", cfg.ValidateCmd)

	fmt.Println()

	// Get config path
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	fmt.Printf("Config file: %s\n", path)

	return nil
}
