package main

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/darkLord19/foglet/internal/ai"
	fogenv "github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/spf13/cobra"
)

var (
	setupDefaultToolFlag string
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Onboard Fog and configure default AI tool",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSetup(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	setupCmd.Flags().StringVar(&setupDefaultToolFlag, "default-tool", "", "Default AI tool (cursor, claude, antigravity)")
	rootCmd.AddCommand(setupCmd)
}

func runSetup() error {
	fogHome, err := fogenv.FogHome()
	if err != nil {
		return err
	}

	store, err := state.NewStore(fogHome)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	fmt.Println("Checking GitHub CLI status...")
	if !ghcli.IsGhAvailable() {
		return fmt.Errorf("gh CLI not found. Please install GitHub CLI (https://cli.github.com/)")
	}
	if !ghcli.IsGhAuthenticated() {
		return fmt.Errorf("gh CLI not authenticated. Please run 'gh auth login'")
	}
	fmt.Println("GitHub CLI is installed and authenticated.")

	available := availableTools()
	if len(available) == 0 {
		return fmt.Errorf("no supported AI tools found in PATH (expected cursor, claude, or antigravity)")
	}

	defaultTool, err := chooseDefaultTool(available, setupDefaultToolFlag)
	if err != nil {
		return err
	}
	if err := store.SetDefaultTool(defaultTool); err != nil {
		return err
	}

	fmt.Println("Setup complete")
	fmt.Printf("Fog home: %s\n", fogHome)
	fmt.Printf("Default tool: %s\n", defaultTool)
	return nil
}

func availableTools() []string {
	names := ai.AvailableToolNames()
	out := make([]string, 0, len(names))
	for _, name := range names {
		tool, err := ai.GetTool(name)
		if err != nil {
			continue
		}
		if tool.IsAvailable() {
			out = append(out, name)
		}
	}
	return out
}

func chooseDefaultTool(available []string, requested string) (string, error) {
	if len(available) == 0 {
		return "", fmt.Errorf("no available tools")
	}

	requested = strings.TrimSpace(requested)
	if requested != "" {
		if slices.Contains(available, requested) {
			return requested, nil
		}
		return "", fmt.Errorf("requested default tool %q is not available, available: %s", requested, strings.Join(available, ", "))
	}

	if len(available) == 1 {
		return available[0], nil
	}

	fmt.Println("Available AI tools:")
	for i, tool := range available {
		fmt.Printf("  %d. %s\n", i+1, tool)
	}

	choice, err := readLine("Select default tool number: ")
	if err != nil {
		return "", err
	}

	idx, err := parseIndex(choice, len(available))
	if err != nil {
		return "", err
	}
	return available[idx], nil
}

func readLine(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
