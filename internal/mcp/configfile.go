package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type mcpHTTPEntry struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// BuildConfigFile resolves credentials for all configured upstreams and writes
// a temporary JSON config file in the format Claude Code's --mcp-config flag
// accepts. Returns ("", no-op, nil) when no upstreams are configured.
// The caller must invoke cleanup() to delete the temp file.
func BuildConfigFile(fogHome string, secrets SecretReader) (path string, cleanup func(), err error) {
	upstreams, err := LoadConfig(fogHome)
	if err != nil {
		return "", func() {}, err
	}
	if len(upstreams) == 0 {
		return "", func() {}, nil
	}

	servers := make(map[string]mcpHTTPEntry, len(upstreams))
	for _, u := range upstreams {
		entry := mcpHTTPEntry{Type: "http", URL: u.URL}
		if u.SecretKey != "" && secrets != nil {
			if token, found, terr := secrets.GetSecret(u.SecretKey); terr == nil && found {
				if t := strings.TrimSpace(token); t != "" {
					entry.Headers = map[string]string{"Authorization": "Bearer " + t}
				}
			}
		}
		servers[u.Name] = entry
	}

	data, err := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	if err != nil {
		return "", func() {}, fmt.Errorf("marshal MCP config: %w", err)
	}

	f, err := os.CreateTemp("", "fog-mcp-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("create MCP temp file: %w", err)
	}
	_ = f.Chmod(0o600)
	if _, werr := f.Write(data); werr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("write MCP temp file: %w", werr)
	}
	if cerr := f.Close(); cerr != nil {
		_ = os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("close MCP temp file: %w", cerr)
	}

	p := f.Name()
	return p, func() { _ = os.Remove(p) }, nil
}
