package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	apiTokenFile = "api.token"
	apiTokenLen  = 32 // 32 bytes → 64 hex chars
)

// GenerateAPIToken creates a cryptographically random hex token.
func GenerateAPIToken() (string, error) {
	buf := make([]byte, apiTokenLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// WriteTokenFile persists the API token to <fogHome>/api.token with mode 0600.
func WriteTokenFile(fogHome, token string) error {
	if err := os.MkdirAll(fogHome, 0o700); err != nil {
		return fmt.Errorf("create fog home: %w", err)
	}
	tokenPath := filepath.Join(fogHome, apiTokenFile)
	return os.WriteFile(tokenPath, []byte(token), 0o600)
}

// ReadTokenFile reads the API token from <fogHome>/api.token.
func ReadTokenFile(fogHome string) (string, error) {
	tokenPath := filepath.Join(fogHome, apiTokenFile)
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read api token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WithAuth returns middleware that enforces Bearer token authentication.
// The /health endpoint and OPTIONS preflight requests are exempt.
// Only /api/* routes require auth so Slack webhooks and other non-API handlers
// can function without needing to attach the bearer token.
func WithAuth(token string, next http.Handler) http.Handler {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exempt health check and CORS preflight.
		if r.URL.Path == "/health" || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Only protect the local API surface.
		if !strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api" {
			next.ServeHTTP(w, r)
			return
		}

		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing or invalid authorization header", http.StatusUnauthorized)
			return
		}
		presented := strings.TrimSpace(auth[len("Bearer "):])
		if subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
			http.Error(w, "invalid api token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
