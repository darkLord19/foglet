package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

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
		fmt.Println("wtx - workspace manager")
		fmt.Println("Run 'wtx --help' for usage")
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
	rootCmd.AddCommand(versionCmd)
}
