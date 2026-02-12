package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/darkLord19/foglet/internal/api"
	"github.com/darkLord19/foglet/internal/cloudcfg"
	"github.com/darkLord19/foglet/internal/cloudrelay"
	"github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/slack"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagPort        int
	flagSlackSecret string
	flagEnableSlack bool
	flagSlackMode   string
	flagSlackBot    string
	flagSlackApp    string
	flagCloudURL    string
	flagCloudPoll   time.Duration
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
	rootCmd.Flags().StringVar(&flagSlackMode, "slack-mode", "http", "Slack integration mode: http or socket")
	rootCmd.Flags().StringVar(&flagSlackBot, "slack-bot-token", "", "Slack bot token (xoxb-..., required for socket mode)")
	rootCmd.Flags().StringVar(&flagSlackApp, "slack-app-token", "", "Slack app token (xapp-..., required for socket mode)")
	rootCmd.Flags().StringVar(&flagCloudURL, "cloud-url", "", "Fog cloud base URL for distributed Slack relay (optional)")
	rootCmd.Flags().DurationVar(&flagCloudPoll, "cloud-poll-interval", 2*time.Second, "Fog cloud relay polling interval")

	rootCmd.AddCommand(versionCmd)
}

func runDaemon() error {
	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

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

	stateStore, err := state.NewStore(configDir)
	if err != nil {
		return err
	}
	defer func() { _ = stateStore.Close() }()

	// Create mux
	mux := http.NewServeMux()

	// Register API routes
	apiServer := api.New(r, stateStore, flagPort)
	apiServer.RegisterRoutes(mux)

	// Register Slack integration if enabled
	if flagEnableSlack {
		mode := strings.ToLower(strings.TrimSpace(flagSlackMode))
		if err := validateSlackConfig(mode, flagSlackBot, flagSlackApp); err != nil {
			return err
		}

		switch mode {
		case "http":
			if strings.TrimSpace(flagSlackSecret) == "" {
				log.Println("Warning: Slack HTTP mode enabled without --slack-secret")
			}

			slackHandler := slack.New(r, stateStore, flagSlackSecret)
			mux.HandleFunc("/slack/command", slackHandler.HandleCommand)

			log.Println("Slack integration enabled (http mode)")
			log.Printf("Slack webhook: http://localhost:%d/slack/command\n", flagPort)
			log.Println("Note: Use a tunnel service (ngrok, cloudflared) to expose this to Slack")

		case "socket":
			socketServer := slack.NewSocketMode(r, stateStore, flagSlackApp, flagSlackBot)
			go func() {
				if err := socketServer.Run(daemonCtx); err != nil {
					log.Printf("Slack socket mode stopped: %v", err)
				}
			}()

			log.Println("Slack integration enabled (socket mode)")
			log.Println("Socket mode active: listening for app mentions and slash commands over WebSocket")

		default:
			return fmt.Errorf("invalid --slack-mode %q: expected http or socket", flagSlackMode)
		}
	}

	cloudURL := strings.TrimSpace(flagCloudURL)
	if cloudURL != "" {
		if err := stateStore.SetSetting(cloudcfg.SettingCloudURL, cloudURL); err != nil {
			return fmt.Errorf("persist cloud url: %w", err)
		}
	} else if stored, found, err := stateStore.GetSetting(cloudcfg.SettingCloudURL); err == nil && found {
		cloudURL = strings.TrimSpace(stored)
	}
	if cloudURL != "" {
		deviceID, foundID, err := stateStore.GetSetting(cloudcfg.SettingCloudDeviceID)
		if err != nil {
			return err
		}
		deviceToken, foundToken, err := stateStore.GetSecret(cloudcfg.SecretCloudDeviceTok)
		if err != nil {
			return err
		}
		if !foundID || strings.TrimSpace(deviceID) == "" || !foundToken || strings.TrimSpace(deviceToken) == "" {
			log.Println("Cloud URL configured but device is not paired yet. Pair via http://localhost:<port>/api/cloud/pair")
		} else {
			client, err := cloudrelay.NewClient(cloudrelay.ClientConfig{
				BaseURL:     cloudURL,
				DeviceID:    strings.TrimSpace(deviceID),
				DeviceToken: strings.TrimSpace(deviceToken),
			})
			if err != nil {
				return err
			}
			relay, err := cloudrelay.New(client, r, stateStore, cloudrelay.RelayConfig{
				PollInterval: flagCloudPoll,
			})
			if err != nil {
				return err
			}
			go func() {
				if err := relay.Run(daemonCtx); err != nil {
					log.Printf("Cloud relay stopped: %v", err)
				}
			}()
			log.Printf("Cloud relay enabled: %s (device: %s)", cloudURL, strings.TrimSpace(deviceID))
		}
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		daemonCancel()
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("Starting fogd on %s\n", addr)
	log.Printf("API: http://localhost:%d/api/\n", flagPort)
	log.Printf("Health: http://localhost:%d/health\n", flagPort)

	return http.ListenAndServe(addr, api.WithCORS(mux))
}

func validateSlackConfig(mode, botToken, appToken string) error {
	switch mode {
	case "http":
		return nil
	case "socket":
		if strings.TrimSpace(botToken) == "" || strings.TrimSpace(appToken) == "" {
			return fmt.Errorf("slack socket mode requires --slack-bot-token and --slack-app-token")
		}
		return nil
	default:
		return fmt.Errorf("invalid --slack-mode %q: expected http or socket", mode)
	}
}
