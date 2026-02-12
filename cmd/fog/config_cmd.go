package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/darkLord19/wtx/internal/ai"
	wtxconfig "github.com/darkLord19/wtx/internal/config"
	fogenv "github.com/darkLord19/wtx/internal/env"
	"github.com/darkLord19/wtx/internal/state"
	"github.com/spf13/cobra"
)

var (
	configJSONFlag      bool
	configSetToolFlag   string
	configSetPrefixFlag string
)

type combinedConfigView struct {
	WTX *wtxconfig.Config `json:"wtx"`
	Fog fogConfigView     `json:"fog"`
}

type fogConfigView struct {
	Home           string `json:"home"`
	ManagedRepos   string `json:"managed_repos_dir"`
	DefaultTool    string `json:"default_tool,omitempty"`
	BranchPrefix   string `json:"branch_prefix,omitempty"`
	HasGitHubToken bool   `json:"has_github_token"`
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update Fog configuration",
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Show combined Fog + wtx configuration",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigView(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update Fog configuration values",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigSet(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	configViewCmd.Flags().BoolVar(&configJSONFlag, "json", false, "Output JSON")
	configSetCmd.Flags().StringVar(&configSetToolFlag, "default-tool", "", "Set Fog default AI tool")
	configSetCmd.Flags().StringVar(&configSetPrefixFlag, "branch-prefix", "", "Set Fog branch prefix")

	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigView() error {
	view, err := loadCombinedConfigView()
	if err != nil {
		return err
	}

	if configJSONFlag {
		data, err := json.MarshalIndent(view, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("wtx config:")
	fmt.Printf("  editor: %s\n", view.WTX.Editor)
	fmt.Printf("  worktree_dir: %s\n", view.WTX.WorktreeDir)
	fmt.Printf("  default_branch: %s\n", view.WTX.DefaultBranch)

	fmt.Println("fog config:")
	fmt.Printf("  home: %s\n", view.Fog.Home)
	fmt.Printf("  managed_repos_dir: %s\n", view.Fog.ManagedRepos)
	fmt.Printf("  default_tool: %s\n", valueOrUnset(view.Fog.DefaultTool))
	fmt.Printf("  branch_prefix: %s\n", valueOrUnset(view.Fog.BranchPrefix))
	fmt.Printf("  github_pat: %s\n", boolLabel(view.Fog.HasGitHubToken))

	return nil
}

func runConfigSet() error {
	tool := strings.TrimSpace(configSetToolFlag)
	prefix := strings.TrimSpace(configSetPrefixFlag)
	if tool == "" && prefix == "" {
		return fmt.Errorf("provide at least one value: --default-tool or --branch-prefix")
	}

	if tool != "" {
		if err := validateToolAvailable(tool); err != nil {
			return err
		}
	}
	if prefix != "" {
		if err := validateBranchPrefix(prefix); err != nil {
			return err
		}
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

	if tool != "" {
		if err := store.SetDefaultTool(tool); err != nil {
			return err
		}
	}
	if prefix != "" {
		if err := store.SetSetting("branch_prefix", prefix); err != nil {
			return err
		}
	}

	fmt.Println("Fog settings updated")
	return runConfigView()
}

func loadCombinedConfigView() (*combinedConfigView, error) {
	wtxCfg, err := wtxconfig.Load()
	if err != nil {
		return nil, err
	}

	fogHome, err := fogenv.FogHome()
	if err != nil {
		return nil, err
	}

	store, err := state.NewStore(fogHome)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()

	fogCfg, err := loadFogConfigView(store, fogHome)
	if err != nil {
		return nil, err
	}

	return &combinedConfigView{
		WTX: wtxCfg,
		Fog: fogCfg,
	}, nil
}

func loadFogConfigView(store *state.Store, fogHome string) (fogConfigView, error) {
	view := fogConfigView{
		Home:         fogHome,
		ManagedRepos: fogenv.ManagedReposDir(fogHome),
	}

	if tool, found, err := store.GetDefaultTool(); err == nil && found {
		view.DefaultTool = tool
	}
	if prefix, found, err := store.GetSetting("branch_prefix"); err == nil && found {
		view.BranchPrefix = prefix
	}
	if hasToken, err := store.HasGitHubToken(); err == nil {
		view.HasGitHubToken = hasToken
	}

	return view, nil
}

func validateToolAvailable(name string) error {
	tool, err := ai.GetTool(name)
	if err != nil {
		return fmt.Errorf("unknown tool %q", name)
	}
	if !tool.IsAvailable() {
		return fmt.Errorf("tool %q is not available in PATH", name)
	}
	return nil
}

func validateBranchPrefix(prefix string) error {
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("branch prefix cannot be empty")
	}
	if len(prefix) > 120 {
		return fmt.Errorf("branch prefix is too long")
	}
	return nil
}

func valueOrUnset(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unset)"
	}
	return value
}

func boolLabel(ok bool) string {
	if ok {
		return "configured"
	}
	return "missing"
}
