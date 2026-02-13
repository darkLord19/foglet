package daemon

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/darkLord19/foglet/internal/api"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
)

const defaultHealthTimeout = 15 * time.Second

type embeddedDaemon struct {
	server     *http.Server
	stateStore *state.Store
}

var (
	embeddedMu      sync.Mutex
	embeddedDaemons = map[int]*embeddedDaemon{}
)

// EnsureRunning checks /health and starts fogd if needed.
// Returns the base URL and the API bearer token.
func EnsureRunning(fogHome string, port int, timeout time.Duration) (string, string, error) {
	if timeout <= 0 {
		timeout = defaultHealthTimeout
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	healthURL := baseURL + "/health"

	// Read existing token if daemon is already running.
	existingToken, _ := api.ReadTokenFile(fogHome)

	if isHealthy(healthURL, 2*time.Second) {
		return baseURL, existingToken, nil
	}

	if err := startFogd(fogHome, port); err != nil {
		return "", "", err
	}

	if err := waitForHealth(healthURL, timeout); err != nil {
		return "", "", err
	}

	// Re-read token that was generated during startFogd.
	token, _ := api.ReadTokenFile(fogHome)
	return baseURL, token, nil
}

func startFogd(fogHome string, port int) error {
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	embeddedMu.Lock()
	defer embeddedMu.Unlock()

	if _, ok := embeddedDaemons[port]; ok {
		return nil
	}
	if isHealthy(healthURL, 2*time.Second) {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = fogHome
	}

	r, err := runner.New(cwd, fogHome)
	if err != nil {
		return fmt.Errorf("create embedded fog runner: %w", err)
	}
	stateStore, err := state.NewStore(fogHome)
	if err != nil {
		return fmt.Errorf("open embedded fog state store: %w", err)
	}

	mux := http.NewServeMux()
	api.New(r, stateStore, port).RegisterRoutes(mux)

	// Generate API token for local auth.
	apiToken, err := api.GenerateAPIToken()
	if err != nil {
		_ = stateStore.Close()
		return fmt.Errorf("generate api token: %w", err)
	}
	if err := api.WriteTokenFile(fogHome, apiToken); err != nil {
		_ = stateStore.Close()
		return fmt.Errorf("write api token: %w", err)
	}

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: api.WithCORS(api.WithBodyLimit(api.WithAuth(apiToken, mux))),
	}
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		_ = stateStore.Close()
		return fmt.Errorf("listen embedded fogd on %s: %w", server.Addr, err)
	}

	embeddedDaemons[port] = &embeddedDaemon{
		server:     server,
		stateStore: stateStore,
	}

	go func() {
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("embedded fogd stopped with error: %v", err)
		}
		embeddedMu.Lock()
		delete(embeddedDaemons, port)
		embeddedMu.Unlock()
		_ = stateStore.Close()
	}()

	return nil
}

func waitForHealth(healthURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isHealthy(healthURL, 2*time.Second) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("fogd health check timed out: %s", healthURL)
}

func isHealthy(healthURL string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	return isHealthyWithClient(client, healthURL)
}

func isHealthyWithClient(client *http.Client, healthURL string) bool {
	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
