// Package mcp provides Fog's local MCP proxy. Agents connect to one local
// endpoint while Fog owns the upstream server registry and credential lookup.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxBody = 2 << 20

// SecretReader resolves a secret by reference. The proxy never persists or
// returns the resolved value.
type SecretReader interface {
	GetSecret(key string) (string, bool, error)
}

// Upstream is the non-secret MCP configuration stored in mcp.json.
type Upstream struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SecretKey string `json:"secret_key,omitempty"`
}

// LoadConfig reads the one local MCP registry. Missing configuration is a
// valid empty registry, which keeps Fog usable before MCP is configured.
func LoadConfig(fogHome string) ([]Upstream, error) {
	path := filepath.Join(strings.TrimSpace(fogHome), "mcp.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read MCP config: %w", err)
	}
	var upstreams []Upstream
	if err := json.Unmarshal(data, &upstreams); err != nil {
		return nil, fmt.Errorf("decode MCP config: %w", err)
	}
	for i := range upstreams {
		upstreams[i].Name = strings.TrimSpace(upstreams[i].Name)
		upstreams[i].URL = strings.TrimSpace(upstreams[i].URL)
		upstreams[i].SecretKey = strings.TrimSpace(upstreams[i].SecretKey)
		if upstreams[i].Name == "" || upstreams[i].URL == "" {
			return nil, fmt.Errorf("MCP upstream %d requires name and url", i)
		}
	}
	return upstreams, nil
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// Proxy implements the small MCP surface required by coding agents and
// forwards tools/list and tools/call to configured streamable HTTP servers.
type Proxy struct {
	upstreams []Upstream
	secrets   SecretReader
	client    *http.Client
}

func NewProxy(upstreams []Upstream, secrets SecretReader) *Proxy {
	copyUpstreams := append([]Upstream(nil), upstreams...)
	return &Proxy{
		upstreams: copyUpstreams,
		secrets:   secrets,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/mcp/tools" {
		p.listToolsHTTP(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONRPCError(w, http.StatusMethodNotAllowed, nil, -32600, "MCP requires POST")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		writeJSONRPCError(w, http.StatusBadRequest, nil, -32600, "invalid request body")
		return
	}
	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Method) == "" {
		writeJSONRPCError(w, http.StatusBadRequest, req.ID, -32600, "invalid JSON-RPC request")
		return
	}

	switch req.Method {
	case "initialize":
		writeJSON(w, http.StatusOK, map[string]any{
			"jsonrpc": "2.0",
			"id":      rawOrNull(req.ID),
			"result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]string{"name": "fog-mcp-proxy", "version": "0.1.0"},
			},
		})
	case "tools/list":
		p.handleList(w, r.Context(), req.ID)
	case "tools/call":
		p.handleCall(w, r.Context(), req.ID, req.Params)
	default:
		writeJSONRPCError(w, http.StatusOK, req.ID, -32601, "method not found")
	}
}

func (p *Proxy) listToolsHTTP(w http.ResponseWriter, r *http.Request) {
	tools, err := p.listTools(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (p *Proxy) handleList(w http.ResponseWriter, ctx context.Context, id json.RawMessage) {
	tools, err := p.listTools(ctx)
	if err != nil {
		writeJSONRPCError(w, http.StatusBadGateway, id, -32001, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": rawOrNull(id), "result": map[string]any{"tools": tools}})
}

func (p *Proxy) listTools(ctx context.Context) ([]map[string]any, error) {
	all := make([]map[string]any, 0)
	for _, upstream := range p.upstreams {
		resp, err := p.callUpstream(ctx, upstream, jsonRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list"})
		if err != nil {
			return nil, fmt.Errorf("list tools from %s: %w", upstream.Name, err)
		}
		var result struct {
			Tools []map[string]any `json:"tools"`
		}
		if len(resp.Result) == 0 || json.Unmarshal(resp.Result, &result) != nil {
			return nil, fmt.Errorf("list tools from %s returned invalid result", upstream.Name)
		}
		for _, tool := range result.Tools {
			name, _ := tool["name"].(string)
			tool["name"] = upstream.Name + "__" + name
			all = append(all, tool)
		}
	}
	return all, nil
}

func (p *Proxy) handleCall(w http.ResponseWriter, ctx context.Context, id json.RawMessage, rawParams json.RawMessage) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &params); err != nil || strings.TrimSpace(params.Name) == "" {
		writeJSONRPCError(w, http.StatusOK, id, -32602, "tools/call requires a tool name")
		return
	}
	parts := strings.SplitN(params.Name, "__", 2)
	if len(parts) != 2 {
		writeJSONRPCError(w, http.StatusOK, id, -32602, "tool name is not namespaced")
		return
	}
	var upstream Upstream
	for _, candidate := range p.upstreams {
		if candidate.Name == parts[0] {
			upstream = candidate
			break
		}
	}
	if upstream.Name == "" {
		writeJSONRPCError(w, http.StatusOK, id, -32602, "unknown MCP upstream")
		return
	}
	paramsBytes, _ := json.Marshal(map[string]any{"name": parts[1], "arguments": params.Arguments})
	resp, err := p.callUpstream(ctx, upstream, jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: "tools/call", Params: paramsBytes})
	if err != nil {
		log.Printf("mcp proxy call failed upstream=%s tool=%s: %v", upstream.Name, parts[1], err)
		writeJSONRPCError(w, http.StatusBadGateway, id, -32001, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (p *Proxy) callUpstream(ctx context.Context, upstream Upstream, request jsonRPCRequest) (jsonRPCResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return jsonRPCResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, strings.NewReader(string(body)))
	if err != nil {
		return jsonRPCResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if upstream.SecretKey != "" && p.secrets != nil {
		secret, found, err := p.secrets.GetSecret(upstream.SecretKey)
		if err != nil {
			return jsonRPCResponse{}, err
		}
		if found && secret != "" {
			req.Header.Set("Authorization", "Bearer "+secret)
		}
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return jsonRPCResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return jsonRPCResponse{}, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}
	var out jsonRPCResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
		return jsonRPCResponse{}, err
	}
	return out, nil
}

func rawOrNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`null`)
	}
	return raw
}

func writeJSONRPCError(w http.ResponseWriter, status int, id json.RawMessage, code int, message string) {
	writeJSON(w, status, map[string]any{"jsonrpc": "2.0", "id": rawOrNull(id), "error": map[string]any{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
