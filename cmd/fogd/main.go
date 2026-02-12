package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/darkLord19/wtx/internal/api"
	"github.com/darkLord19/wtx/internal/env"
	"github.com/darkLord19/wtx/internal/runner"
	"github.com/darkLord19/wtx/internal/slack"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagPort        int
	flagSlackSecret string
	flagEnableSlack bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "fogd",
	Short: "Fog daemon - control plane for AI agents",
	Long:  `fogd provides HTTP API and Slack integration for Fog AI orchestration`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDaemon(); err != nil {
			log.Fatal(err)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("fogd version %s\n", version)
	},
}

func init() {
	rootCmd.Flags().IntVar(&flagPort, "port", 8080, "HTTP server port")
	rootCmd.Flags().StringVar(&flagSlackSecret, "slack-secret", "", "Slack signing secret")
	rootCmd.Flags().BoolVar(&flagEnableSlack, "enable-slack", false, "Enable Slack integration")

	rootCmd.AddCommand(versionCmd)
}

func runDaemon() error {
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

	// Create runner
	r, err := runner.New(cwd, configDir)
	if err != nil {
		return err
	}

	// Create mux
	mux := http.NewServeMux()

	// Register API routes
	apiServer := api.New(r, flagPort)
	apiServer.RegisterRoutes(mux)

	// Register Slack integration if enabled
	if flagEnableSlack {
		if flagSlackSecret == "" {
			log.Println("Warning: Slack enabled but no signing secret provided")
		}

		slackHandler := slack.New(r, flagSlackSecret)
		mux.HandleFunc("/slack/command", slackHandler.HandleCommand)

		log.Println("Slack integration enabled")
		log.Printf("Slack webhook: http://localhost:%d/slack/command\n", flagPort)
		log.Println("Note: Use a tunnel service (ngrok, cloudflared) to expose this to Slack")
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("Starting fogd on %s\n", addr)
	log.Printf("API: http://localhost:%d/api/\n", flagPort)
	log.Printf("Health: http://localhost:%d/health\n", flagPort)

	return http.ListenAndServe(addr, mux)
}
