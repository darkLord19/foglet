package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/wtx/internal/config"
	"github.com/yourusername/wtx/internal/editor"
	"github.com/yourusername/wtx/internal/git"
	"github.com/yourusername/wtx/internal/metadata"
)

var version = "dev"

var (
	flagJSON   bool
	flagEditor string
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
		// TODO: Launch TUI
		fmt.Println("wtx - workspace manager")
		fmt.Println("Run 'wtx list' to see all worktrees")
		fmt.Println("Run 'wtx --help' for usage")
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
	
	rootCmd.PersistentFlags().StringVar(&flagEditor, "editor", "", "Editor to use (vscode, cursor, neovim, etc)")
	
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(rmCmd)
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
	fmt.Printf("Creating worktree '%s' at %s...\n", name, wtPath)
	if err := g.AddWorktree(wtPath, branch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
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
	
	fmt.Printf("✓ Worktree '%s' created\n", name)
	
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
