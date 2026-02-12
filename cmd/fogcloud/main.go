package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/darkLord19/wtx/internal/cloud"
	"github.com/darkLord19/wtx/internal/env"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagPort              int
	flagDataDir           string
	flagPublicURL         string
	flagSlackClientID     string
	flagSlackClientSecret string
	flagSlackSigning      string
	flagSlackScopes       string
	flagPairCodeTTL       time.Duration
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "fogcloud",
	Short: "Fog cloud control plane for Slack distribution",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("fogcloud version %s\n", version)
	},
}

func init() {
	rootCmd.Flags().IntVar(&flagPort, "port", 9090, "HTTP server port")
	rootCmd.Flags().StringVar(&flagDataDir, "data-dir", "", "Data directory for cloud sqlite/key (default: $FOG_HOME/cloud)")
	rootCmd.Flags().StringVar(&flagPublicURL, "public-url", "", "Public base URL for Slack OAuth callbacks (required)")
	rootCmd.Flags().StringVar(&flagSlackClientID, "slack-client-id", "", "Slack app client ID (required)")
	rootCmd.Flags().StringVar(&flagSlackClientSecret, "slack-client-secret", "", "Slack app client secret (required)")
	rootCmd.Flags().StringVar(&flagSlackSigning, "slack-signing-secret", "", "Slack signing secret (required)")
	rootCmd.Flags().StringVar(&flagSlackScopes, "slack-scopes", "app_mentions:read,chat:write", "Comma-separated Slack OAuth bot scopes")
	rootCmd.Flags().DurationVar(&flagPairCodeTTL, "pair-code-ttl", 10*time.Minute, "Pairing code TTL")
	rootCmd.AddCommand(versionCmd)
}

func runServer() error {
	dataDir := strings.TrimSpace(flagDataDir)
	if dataDir == "" {
		fogHome, err := env.FogHome()
		if err != nil {
			return err
		}
		dataDir = filepath.Join(fogHome, "cloud")
	}
	scopes := normalizeScopes(flagSlackScopes)
	if len(scopes) == 0 {
		return fmt.Errorf("at least one slack scope is required")
	}

	store, err := cloud.NewStore(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	server, err := cloud.NewServer(store, cloud.Config{
		ClientID:       strings.TrimSpace(flagSlackClientID),
		ClientSecret:   strings.TrimSpace(flagSlackClientSecret),
		SigningSecret:  strings.TrimSpace(flagSlackSigning),
		PublicURL:      strings.TrimSpace(flagPublicURL),
		Scopes:         scopes,
		PairingCodeTTL: flagPairCodeTTL,
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("Starting fogcloud on %s", addr)
	log.Printf("Health: http://localhost:%d/health", flagPort)
	log.Printf("Slack install: %s/slack/install", strings.TrimRight(strings.TrimSpace(flagPublicURL), "/"))
	return http.ListenAndServe(addr, mux)
}

func normalizeScopes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope == "" {
			continue
		}
		out = append(out, scope)
	}
	return out
}
