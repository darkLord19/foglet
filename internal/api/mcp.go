package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/mcp"
)

// MCPUpstreamInfo is the safe public view of a configured MCP upstream.
type MCPUpstreamInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	HasToken bool   `json:"has_token"`
}

// MCPConfigResponse is the response body for GET /api/mcp.
type MCPConfigResponse struct {
	Upstreams []MCPUpstreamInfo `json:"upstreams"`
}

// UpdateMCPUpstream is one entry in the PUT /api/mcp request body.
type UpdateMCPUpstream struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token,omitempty"` // blank keeps the stored token
}

// UpdateMCPRequest is the PUT /api/mcp request body.
type UpdateMCPRequest struct {
	Upstreams []UpdateMCPUpstream `json:"upstreams"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getMCP(w)
	case http.MethodPut:
		s.updateMCP(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMCPGitHubConnect auto-connects the GitHub MCP preset using the token
// already stored by the gh CLI — no manual token entry required.
func (s *Server) handleMCPGitHubConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.mcpProxy == nil || s.fogHome == "" {
		http.Error(w, "MCP not configured", http.StatusServiceUnavailable)
		return
	}

	token, found, err := ghcli.GetAuthToken()
	if err != nil || !found {
		http.Error(w, "gh CLI is not authenticated — run `gh auth login` first", http.StatusBadRequest)
		return
	}

	const githubName = "github"
	const githubURL = "https://api.githubcopilot.com/mcp/"
	const secretKey = "mcp_token_github"

	if err := s.stateStore.SaveSecret(secretKey, token); err != nil {
		http.Error(w, fmt.Sprintf("save token: %v", err), http.StatusInternalServerError)
		return
	}

	// Upsert the github upstream: replace if it exists, append if not.
	current := s.mcpProxy.Upstreams()
	found = false
	newList := make([]mcp.Upstream, 0, len(current)+1)
	for _, u := range current {
		if u.Name == githubName {
			newList = append(newList, mcp.Upstream{Name: githubName, URL: githubURL, SecretKey: secretKey})
			found = true
		} else {
			newList = append(newList, u)
		}
	}
	if !found {
		newList = append(newList, mcp.Upstream{Name: githubName, URL: githubURL, SecretKey: secretKey})
	}

	if err := mcp.SaveConfig(s.fogHome, newList); err != nil {
		http.Error(w, fmt.Sprintf("save MCP config: %v", err), http.StatusInternalServerError)
		return
	}
	s.mcpProxy.SetUpstreams(newList)

	s.getMCP(w)
}

func (s *Server) getMCP(w http.ResponseWriter) {
	if s.mcpProxy == nil {
		s.writeJSON(w, http.StatusOK, MCPConfigResponse{Upstreams: []MCPUpstreamInfo{}})
		return
	}
	upstreams := s.mcpProxy.Upstreams()
	infos := make([]MCPUpstreamInfo, 0, len(upstreams))
	for _, u := range upstreams {
		has := false
		if u.SecretKey != "" {
			has, _ = s.stateStore.HasSecret(u.SecretKey)
		}
		infos = append(infos, MCPUpstreamInfo{
			Name:     u.Name,
			URL:      u.URL,
			HasToken: has,
		})
	}
	s.writeJSON(w, http.StatusOK, MCPConfigResponse{Upstreams: infos})
}

func (s *Server) updateMCP(w http.ResponseWriter, r *http.Request) {
	if s.mcpProxy == nil || s.fogHome == "" {
		http.Error(w, "MCP not configured", http.StatusServiceUnavailable)
		return
	}

	var req UpdateMCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate: no empty names, no __ in names, no dups, http/https URLs only.
	seen := make(map[string]bool, len(req.Upstreams))
	for i, u := range req.Upstreams {
		name := strings.TrimSpace(u.Name)
		if name == "" {
			http.Error(w, fmt.Sprintf("upstream %d: name is required", i), http.StatusBadRequest)
			return
		}
		if strings.Contains(name, "__") {
			http.Error(w, fmt.Sprintf("upstream %q: name must not contain __", name), http.StatusBadRequest)
			return
		}
		if seen[name] {
			http.Error(w, fmt.Sprintf("duplicate upstream name %q", name), http.StatusBadRequest)
			return
		}
		seen[name] = true
		rawURL := strings.TrimSpace(u.URL)
		if rawURL == "" {
			http.Error(w, fmt.Sprintf("upstream %q: url is required", name), http.StatusBadRequest)
			return
		}
		lower := strings.ToLower(rawURL)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			http.Error(w, fmt.Sprintf("upstream %q: url must be http or https", name), http.StatusBadRequest)
			return
		}
	}

	// Determine which names are being removed so we can delete their secrets.
	oldUpstreams := s.mcpProxy.Upstreams()
	oldNames := make(map[string]string, len(oldUpstreams)) // name -> secret_key
	for _, u := range oldUpstreams {
		oldNames[u.Name] = u.SecretKey
	}

	newNames := make(map[string]bool, len(req.Upstreams))
	for _, u := range req.Upstreams {
		newNames[strings.TrimSpace(u.Name)] = true
	}
	for oldName, secretKey := range oldNames {
		if !newNames[oldName] && secretKey != "" {
			_ = s.stateStore.DeleteSecret(secretKey)
		}
	}

	// Process tokens and build the new upstream list.
	newUpstreams := make([]mcp.Upstream, 0, len(req.Upstreams))
	for _, u := range req.Upstreams {
		name := strings.TrimSpace(u.Name)
		rawURL := strings.TrimSpace(u.URL)
		token := strings.TrimSpace(u.Token)

		secretKey := "mcp_token_" + name
		if token != "" {
			if err := s.stateStore.SaveSecret(secretKey, token); err != nil {
				http.Error(w, fmt.Sprintf("save token for %q: %v", name, err), http.StatusInternalServerError)
				return
			}
		}

		// Determine whether a secret key should be set in the config.
		hasSecret := false
		if token != "" {
			hasSecret = true
		} else if existing, _ := s.stateStore.HasSecret(secretKey); existing {
			hasSecret = true
		}

		up := mcp.Upstream{Name: name, URL: rawURL}
		if hasSecret {
			up.SecretKey = secretKey
		}
		newUpstreams = append(newUpstreams, up)
	}

	if err := mcp.SaveConfig(s.fogHome, newUpstreams); err != nil {
		http.Error(w, fmt.Sprintf("save MCP config: %v", err), http.StatusInternalServerError)
		return
	}
	s.mcpProxy.SetUpstreams(newUpstreams)

	s.getMCP(w)
}
